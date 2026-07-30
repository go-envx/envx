package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/manifest"
	"github.com/go-envx/envx/app/internal/runner"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/internal/secrets"
	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// Input is the raw user input one action gathers at the frontend edge (a cobra
// command today, an HTTP handler tomorrow). Each setting is optional: a non-nil
// value means the user provided it explicitly and it wins the precedence chain,
// while nil means "fall through" to the ENVX_* var and then the manifest layers.
// ConfigPath selects the manifest. ResolveProject and ResolveWorkspace turn it
// into a *Result.
type Input struct {
	// ConfigPath selects the manifest file; nil or empty triggers auto-discovery.
	ConfigPath *string
	// Env is the explicitly requested target environment.
	Env *string
	// RequireOverlays, when set, requires every overlay file in the chain to exist.
	RequireOverlays *bool
	// Prefix is the explicitly requested key prefix.
	Prefix *string
	// Suffix is the explicitly requested key suffix.
	Suffix *string
	// Delimiter is the explicitly requested list-join delimiter.
	Delimiter *string
	// NamespacePrefix, when set, prefixes each key with its namespace.
	NamespacePrefix *bool
	// Overload, when set, lets file values win over existing OS env vars.
	Overload *bool
}

// -------------------------------------------------------------------------------------

// manifestContext bundles the loaded manifest, the directory it was loaded from,
// and the project being resolved: the shared input resolveManifest and
// resolveProjectLayer both read, and which Result retains so OverlayPath can
// validate and join a target without re-loading.
type manifestContext struct {
	// manifest is the parsed, validated manifest.
	manifest *schema.Manifest
	// dir is the absolute directory the manifest was loaded from.
	dir string
	// project is the project name being resolved ("" resolves the global context).
	project string
}

// -------------------------------------------------------------------------------------

// projectLayer is the project's contribution to resolution: its setting overrides
// and its includes resolved to absolute paths. The zero value is the global-only
// context — no overrides and no includes.
type projectLayer struct {
	// settings are the project-level setting overrides layered over the global ones.
	settings schema.Settings
	// includes are the project's namespaces resolved to absolute paths.
	includes []string
}

// -------------------------------------------------------------------------------------

// ResolveProject resolves a project's build-ready configuration: it loads the
// manifest, meshes it with the input and ENVX_* vars, then opens the workspace
// secrets store and wires the value resolver so envmerge can dereference
// secret:// references. The environment-building actions (get, run, explain,
// diff) call it. A missing store yields an empty resolver, so a reference with no
// matching entry fails loudly as a dangling reference rather than leaking the raw
// reference string.
func ResolveProject(in *Input, project string) (*Result, error) {
	res, err := resolve(in, project)
	if err != nil {
		return nil, err
	}

	// Open the workspace secrets store and wire the resolver onto the envmerge
	// params so secret:// references dereference during Build.
	resolver, err := secrets.Open(res.Secrets)
	if err != nil {
		return nil, err
	}
	res.Envmerge.ValueResolver = resolver
	return res, nil
}

// -------------------------------------------------------------------------------------

// ResolveWorkspace resolves manifest-level configuration without selecting a
// project or opening the secrets store, leaving the value resolver nil. The set
// action calls it to locate and edit a single overlay file, which needs no
// project merge and no secrets I/O.
func ResolveWorkspace(in *Input) (*Result, error) {
	return resolve(in, "")
}

// -------------------------------------------------------------------------------------

// resolve is the shared core of ResolveProject and ResolveWorkspace: it loads the
// manifest (honoring --config, then ENVX_CONFIG, then a walk-up search) and
// meshes it with the input's values and ENVX_* vars into a single *Result,
// applying the precedence explicit > ENVX_* > project > global. An empty project
// resolves the global context only (no project layer, no includes, and no
// "project not found" error). Terminal fallbacks (e.g. the default environment)
// are applied downstream, so an unset env stays empty here.
func resolve(in *Input, project string) (*Result, error) {
	manifestPath := resolveManifestPath(in)

	// Load the manifest from the resolved manifest path.
	m, manifestFile, err := manifest.Load(manifestPath)
	if err != nil {
		return nil, err
	}

	// Get the absolute directory of the manifest so project includes can be joined.
	dir := filepath.Dir(manifestFile)

	// Construct the manifest context.
	mc := manifestContext{
		manifest: m,
		dir:      dir,
		project:  project,
	}

	// Resolve the manifest context and input into a single Result.
	return resolveManifest(mc, in)
}

// -------------------------------------------------------------------------------------

// resolveManifestPath resolves where the manifest lives, honoring the precedence
// --config flag > ENVX_CONFIG env var > "" (an empty result lets the manifest
// package walk up from the working directory).
func resolveManifestPath(in *Input) string {
	if in.ConfigPath != nil && *in.ConfigPath != "" {
		return *in.ConfigPath
	}
	if v := os.Getenv(schema.Config.Env); v != "" {
		return v
	}
	return ""
}

// -------------------------------------------------------------------------------------

// resolveManifest assembles a *Result from an already-loaded manifest: it computes
// the project layer, then delegates to the envmerge and runner param builders that
// layer each setting through the precedence chain. It is split from resolve so the
// precedence stays unit-testable with an in-memory manifest.
func resolveManifest(mc manifestContext, in *Input) (*Result, error) {
	// Compute the project layer (settings + includes) from the manifest context.
	pl, err := resolveProjectLayer(mc)
	if err != nil {
		return nil, err
	}

	// Build the config Result from the manifest context, Input, and project layer.
	return &Result{
		Envmerge:        resolveEnvmergeParams(mc, in, pl),
		Runner:          resolveRunnerParams(mc, in, pl),
		Secrets:         resolveSecretsSettings(mc),
		manifestContext: mc,
	}, nil
}

// -------------------------------------------------------------------------------------

// resolveProjectLayer computes the project layer from the manifest context. An
// empty project yields the zero layer (the global-only context); a named project
// absent from the manifest is an error.
func resolveProjectLayer(mc manifestContext) (projectLayer, error) {
	// Empty project yields the zero layer (the global-only context).
	if mc.project == "" {
		return projectLayer{}, nil
	}

	// Look up the project in the manifest.
	project, ok := mc.manifest.LookupProject(mc.project)
	if !ok {
		return projectLayer{}, fmt.Errorf(
			"project %q not found in manifest", mc.project,
		)
	}

	// Resolve the project's includes to absolute paths.
	includes := make([]string, len(project.Includes))
	for i, inc := range project.Includes {
		includes[i] = filepath.Join(mc.dir, inc)
	}

	// Return the project layer: its settings and its includes.
	return projectLayer{
		settings: project.Settings,
		includes: includes,
	}, nil
}

// -------------------------------------------------------------------------------------

// resolveEnvmergeParams builds the envmerge input: the project's includes, the
// declared environments, and every setting layered through the precedence chain
// explicit (input) > ENVX_* > project > global. Terminal defaults (such as the
// first-declared environment) are left to envmerge downstream.
func resolveEnvmergeParams(
	mc manifestContext,
	in *Input,
	pl projectLayer,
) envmerge.Params {
	proj, global := pl.settings, mc.manifest.Settings
	return envmerge.Params{
		Includes:     pl.includes,
		Environments: mc.manifest.Environments,
		Settings: envmerge.Settings{
			Env: precedenceString(&schema.Env,
				in.Env,
				proj.Env,
				global.Env,
			),
			RequireOverlays: precedenceBool(&schema.RequireOverlays,
				in.RequireOverlays,
				proj.RequireOverlays,
				global.RequireOverlays,
			),
			Prefix: precedenceString(&schema.Prefix,
				in.Prefix,
				proj.Prefix,
				global.Prefix,
			),
			Suffix: precedenceString(&schema.Suffix,
				in.Suffix,
				proj.Suffix,
				global.Suffix,
			),
			Delimiter: precedenceString(&schema.Delimiter,
				in.Delimiter,
				proj.Delimiter,
				global.Delimiter,
			),
			NamespacePrefix: precedenceBool(&schema.NamespacePrefix,
				in.NamespacePrefix,
				proj.NamespacePrefix,
				global.NamespacePrefix,
			),
		},
	}
}

// -------------------------------------------------------------------------------------

// resolveRunnerParams builds the runner input: the overload knob layered through
// the precedence chain explicit (input) > ENVX_OVERLOAD > project > global.
func resolveRunnerParams(
	mc manifestContext,
	in *Input,
	pl projectLayer,
) runner.Params {
	return runner.Params{
		Overload: precedenceBool(&schema.Overload,
			in.Overload,
			pl.settings.Overload,
			mc.manifest.Settings.Overload,
		),
	}
}

// -------------------------------------------------------------------------------------

// resolveSecretsSettings builds the secrets input: the resolved workspace store
// path. Secrets are workspace-level — not project- or flag-overridable — so it
// reads only the workspace-level manifest secrets block; opening the store is
// ResolveProject's job.
func resolveSecretsSettings(mc manifestContext) secrets.Settings {
	// Look up the secrets path in the manifest; default to "secrets.yaml" if unset.
	secretsPath := mc.manifest.Secrets.SecretsPath
	if secretsPath == "" {
		secretsPath = "secrets.yaml"
	}

	// Return the secrets settings.
	return secrets.Settings{
		SecretsPath: file.ResolvePath(mc.dir, secretsPath),
	}
}
