package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/manifest"
	"github.com/go-envx/envx/app/internal/runner"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/internal/secrets"
	"github.com/go-envx/envx/app/pkg/file"
)

const (
	// defaultManifestFilename is the conventional workspace manifest filename.
	defaultManifestFilename = "envx.yaml"
	// defaultSecretsFilename is the default workspace secrets store filename.
	defaultSecretsFilename = "secrets.yaml"
	// defaultKeysFilename is the default workspace private-key filename.
	defaultKeysFilename = "envx.keys"
	// defaultCipherAlgorithm is the application's default encryption algorithm.
	defaultCipherAlgorithm = cipher.Age
	// defaultIndent is the block indentation used for yaml files.
	defaultIndent = 2
)

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

// manifestContext bundles the loaded manifest, the directory it was loaded from,
// and the project being resolved: the shared input resolveManifest and
// resolveProjectLayer both read, and which Result retains so OverlayPath can
// validate and join a target without re-loading.
type manifestContext struct {
	// manifest is the parsed, validated manifest.
	manifest *schema.Manifest
	// dir is the absolute directory the manifest was loaded from.
	dir string
	// indent is the block indentation detected in the manifest source document.
	indent int
	// project is the project name being resolved ("" resolves the global context).
	project string
}

// projectLayer is the project's contribution to resolution: its setting overrides
// and its includes resolved to absolute paths. The zero value is the global-only
// context — no overrides and no includes.
type projectLayer struct {
	// settings are the project-level setting overrides layered over the global ones.
	settings schema.Settings
	// includes are the project's namespaces resolved to absolute paths.
	includes []string
}

// ResolveProject resolves a project's build-ready configuration: it loads the
// manifest, meshes it with the input and ENVX_* vars, binds a resolver factory,
// and constructs the envmerge Manager the environment-building actions (get, run,
// explain, diff) operate through. Construction performs no namespace or secrets
// I/O: each Manager operation opens a fresh, operation-scoped resolver from the
// factory and selects its own environment and reveal policy, so run reveals while
// the read commands mask by default. A missing store yields an empty resolver, so
// a reference with no matching entry fails loudly as a dangling reference rather
// than leaking the raw reference string.
func ResolveProject(in *Input, project string) (*Result, error) {
	res, params, err := resolve(in, project)
	if err != nil {
		return nil, err
	}

	// Bind a resolver factory so a Manager operation can open a fresh,
	// operation-scoped resolver on demand without construction-time secrets I/O.
	params.ResolverFactory = resolverFactory{
		secrets: res.Secrets,
		cipher:  res.Cipher,
	}

	// Construct the Manager from the resolved params. New validates and copies the
	// params without reading namespace files or opening the store.
	manager, err := envmerge.New(params)
	if err != nil {
		return nil, err
	}
	res.Envmerge = manager
	return res, nil
}

// resolverFactory lazily constructs a fresh secrets manager and resolver for one
// resolving envmerge operation under the requested reveal policy. It holds only
// construction params, so no store I/O, cipher construction, or private-key
// resolution happens until an operation asks for a resolver — keeping the store
// snapshot and private-key cache operation-scoped. It is the config-owned adapter
// that implements envmerge.ValueResolverFactory, preserving the dependency
// direction in which envmerge defines the consumed interface and config composes
// the provider.
type resolverFactory struct {
	// secrets locates the workspace secrets store and private-key file.
	secrets secrets.Params
	// cipher holds the configured cipher construction parameters.
	cipher cipher.Params
}

// Resolver constructs a fresh secrets manager and opens an operation-scoped
// resolver under the reveal policy.
func (f resolverFactory) Resolver(reveal bool) (envmerge.ValueResolver, error) {
	manager, err := NewSecretsManager(f.secrets, f.cipher)
	if err != nil {
		return nil, err
	}
	resolver, err := manager.Resolver(secrets.ResolverParams{Reveal: reveal})
	if err != nil {
		return nil, err
	}
	return resolver, nil
}

// ResolveWorkspace resolves manifest-level configuration without selecting a
// project, opening the secrets store, or constructing an envmerge Manager,
// leaving Result.Envmerge nil. The set action calls it to locate and edit a
// single overlay file, which needs no project merge and no secrets I/O.
func ResolveWorkspace(in *Input) (*Result, error) {
	res, _, err := resolve(in, "")
	return res, err
}

// resolve is the shared core of ResolveProject and ResolveWorkspace: it loads the
// manifest (honoring --config, then ENVX_CONFIG, then a walk-up search) and
// meshes it with the input's values and ENVX_* vars into a single *Result,
// applying the precedence explicit > ENVX_* > project > global. An empty project
// resolves the global context only (no project layer, no includes, and no
// "project not found" error). Terminal fallbacks (e.g. the default environment)
// are applied downstream, so an unset env stays empty here.
func resolve(in *Input, project string) (*Result, envmerge.Params, error) {
	// Bind the resolved manifest path and conventional filename into a manager.
	manifestManager, err := manifest.New(manifest.Params{
		Path:     resolveManifestPath(in),
		Filename: defaultManifestFilename,
	})
	if err != nil {
		return nil, envmerge.Params{}, err
	}

	// Load the manifest from the resolved manifest path.
	manifestDocument, err := manifestManager.Load()
	if err != nil {
		return nil, envmerge.Params{}, err
	}

	// Get the absolute directory of the manifest so project includes can be joined.
	dir := filepath.Dir(manifestDocument.Path)

	// Construct the manifest context.
	mc := manifestContext{
		manifest: manifestDocument.Content,
		dir:      dir,
		indent:   manifestDocument.Indent,
		project:  project,
	}

	// Resolve the manifest context and input into a single Result.
	return resolveManifest(mc, in)
}

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

// resolveManifest assembles a *Result from an already-loaded manifest: it computes
// the project layer, then delegates to the envmerge and runner param builders that
// layer each setting through the precedence chain. It returns the resolved
// envmerge params alongside the Result so ResolveProject can construct the
// Manager while ResolveWorkspace discards them. It is split from resolve so the
// precedence stays unit-testable with an in-memory manifest.
func resolveManifest(mc manifestContext, in *Input) (*Result, envmerge.Params, error) {
	// Compute the project layer (settings + includes) from the manifest context.
	pl, err := resolveProjectLayer(mc)
	if err != nil {
		return nil, envmerge.Params{}, err
	}

	// Resolve the envmerge params; ResolveProject builds a Manager from them.
	params := resolveEnvmergeParams(mc, in, pl)

	// Build the config Result from the manifest context, Input, and project layer.
	// Envmerge is left nil here; ResolveProject constructs and assigns the Manager.
	return &Result{
		Runner:             resolveRunnerParams(mc, in, pl),
		Secrets:            resolveSecretsParams(mc),
		Cipher:             resolveCipherParams(mc),
		defaultEnvironment: params.DefaultEnvironment,
		manifestContext:    mc,
	}, params, nil
}

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
		DefaultEnvironment: precedenceString(&schema.Env,
			in.Env,
			proj.Env,
			global.Env,
		),
		Settings: envmerge.Settings{
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

// resolveSecretsParams builds the secrets input: the resolved workspace store
// and private-key paths. Secrets are workspace-level — not project- or
// flag-overridable — so it reads only the workspace-level manifest secrets
// block; constructing the manager and opening the store are ResolveProject's
// jobs.
func resolveSecretsParams(mc manifestContext) secrets.Params {
	// Look up the secrets path in the manifest; use the default filename if unset.
	secretsPath := mc.manifest.Secrets.SecretsPath
	if secretsPath == "" {
		secretsPath = defaultSecretsFilename
	}
	resolvedSecretsPath := file.ResolvePath(mc.dir, secretsPath)

	// Look up the private-key path in the manifest; default beside the resolved
	// secrets store and resolve explicit relative paths beside the manifest.
	keysPath := mc.manifest.Secrets.KeysPath
	if keysPath == "" {
		keysPath = filepath.Join(filepath.Dir(resolvedSecretsPath), defaultKeysFilename)
	} else {
		keysPath = file.ResolvePath(mc.dir, keysPath)
	}

	// Resolve the secrets default indent from the manifest's own detected
	// indentation, applying the workspace default when the manifest has none.
	indent := mc.indent
	if indent < 2 || indent > 9 {
		indent = defaultIndent
	}

	// Return the secrets parameters. DefaultIndent is applied only when the
	// secrets store has no block indentation of its own.
	return secrets.Params{
		SecretsPath:   resolvedSecretsPath,
		KeysPath:      keysPath,
		DefaultIndent: indent,
	}
}

// resolveCipherParams resolves the configured algorithm and its construction
// options while keeping cipher selection outside the secrets package.
func resolveCipherParams(mc manifestContext) cipher.Params {
	algorithm := cipher.Algorithm(mc.manifest.Secrets.Cipher)
	if algorithm == "" {
		algorithm = defaultCipherAlgorithm
	}
	return cipher.Params{Algorithm: algorithm}
}
