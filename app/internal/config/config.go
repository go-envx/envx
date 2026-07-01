package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/manifest"
	"github.com/go-envx/envx/app/internal/runner"
	"github.com/go-envx/envx/app/internal/schema"
)

// -------------------------------------------------------------------------------------

// Input is the raw user input one action gathers at the frontend edge (a cobra
// command today, an HTTP handler tomorrow). Each setting is optional: a non-nil
// value means the user provided it explicitly and it wins the precedence chain,
// while nil means "fall through" to the ENVX_* var and then the manifest layers.
// ConfigPath selects the manifest. Resolve turns it into a *Result.
type Input struct {
	// ConfigPath selects the manifest file; nil or empty triggers auto-discovery.
	ConfigPath *string
	// Env is the explicitly requested target environment.
	Env *string
	// Strict, when set, requires every overlay file in the chain to exist.
	Strict *bool
	// Prefix is the explicitly requested key prefix.
	Prefix *string
	// Suffix is the explicitly requested key suffix.
	Suffix *string
	// NamespacePrefix, when set, prefixes each key with its namespace.
	NamespacePrefix *bool
	// Overload, when set, lets file values win over existing OS env vars.
	Overload *bool
}

// -------------------------------------------------------------------------------------

// manifestContext bundles the loaded manifest, the directory it was loaded from,
// and the project being resolved: the shared input resolveManifest and projectLayer
// both read, and which Result retains so OverlayPath can validate and join a target
// without re-loading.
type manifestContext struct {
	// manifest is the parsed, validated manifest.
	manifest *schema.Manifest
	// dir is the absolute directory the manifest was loaded from.
	dir string
	// project is the project name being resolved ("" resolves the global context).
	project string
}

// -------------------------------------------------------------------------------------

// Resolve loads the manifest (honoring --config, then ENVX_CONFIG, then a walk-up
// search) and meshes it with the input's values and ENVX_* vars into a single
// *Result, applying the precedence explicit > ENVX_* > project > global. An
// empty project resolves the global context only (no project layer, no includes,
// and no "project not found" error) for actions that never merge a project.
// Terminal fallbacks (e.g. the default environment) are applied downstream, so an
// unset env stays empty here.
func Resolve(in *Input, project string) (*Result, error) {
	manifestPath := resolveManifestPath(in)

	// Load the manifest from the resolved manifest path.
	m, manifestFile, err := manifest.Load(manifestPath)
	if err != nil {
		return nil, err
	}

	// Get the absolute directory of the manifest so project includes can be joined.
	dir := filepath.Dir(manifestFile)

	// Construct the manifest context
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

// resolveManifest assembles a *Result from an already-loaded manifest: it
// computes the project layer (settings + includes), then layers every setting
// through the precedence chain. It is split from Resolve so the precedence stays
// unit-testable with an in-memory manifest.
func resolveManifest(mc manifestContext, in *Input) (*Result, error) {
	// Compute the project layer (settings + includes) from the manifest context.
	proj, includes, err := projectLayer(mc)
	if err != nil {
		return nil, err
	}

	p := newPrecedence()
	return &Result{
		Envmerge: envmerge.Params{
			Includes:     includes,
			Environments: mc.manifest.Environments,
			Settings:     resolveSettings(p, in, proj, mc.manifest.Settings),
		},
		Runner: runner.Params{
			Overload: p.Bool(
				&schema.Overload, in.Overload, proj.Overload, mc.manifest.Settings.Overload,
			),
		},
		manifestContext: mc,
	}, nil
}

// -------------------------------------------------------------------------------------

// projectLayer returns the project's settings and its includes resolved to
// absolute paths. An empty project contributes the zero settings and no includes
// (the global-only context); a named project absent from the manifest is an error.
func projectLayer(mc manifestContext) (schema.Settings, []string, error) {
	// Empty project returns zero settings and no includes (the global-only context).
	if mc.project == "" {
		return schema.Settings{}, nil, nil
	}

	// Look up the project in the manifest.
	p, ok := mc.manifest.LookupProject(mc.project)
	if !ok {
		return schema.Settings{}, nil, fmt.Errorf(
			"project %q not found in manifest", mc.project,
		)
	}

	// Resolve the project's includes to absolute paths.
	includes := make([]string, len(p.Includes))
	for i, inc := range p.Includes {
		includes[i] = filepath.Join(mc.dir, inc)
	}

	// Return the project's settings and resolved includes.
	return p.Settings, includes, nil
}

// -------------------------------------------------------------------------------------

// resolveSettings layers each envmerge setting through the precedence chain
// explicit (input) > ENVX_* > project > global, leaving terminal defaults (such as
// the first-declared environment) to envmerge downstream.
func resolveSettings(
	p *precedence, in *Input, proj, global schema.Settings,
) envmerge.Settings {
	return envmerge.Settings{
		Env:    p.String(&schema.Env, in.Env, proj.Env, global.Env),
		Strict: p.Bool(&schema.Strict, in.Strict, proj.Strict, global.Strict),
		Prefix: p.String(&schema.Prefix, in.Prefix, proj.Prefix, global.Prefix),
		Suffix: p.String(&schema.Suffix, in.Suffix, proj.Suffix, global.Suffix),
		NamespacePrefix: p.Bool(
			&schema.NamespacePrefix, in.NamespacePrefix,
			proj.NamespacePrefix, global.NamespacePrefix,
		),
	}
}
