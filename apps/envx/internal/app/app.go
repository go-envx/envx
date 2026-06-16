// Package app provides the core application pipeline for envx. It orchestrates
// the shared stages that most commands need: manifest discovery → project
// lookup → config resolution → namespace building → environment merging.
//
// Commands in internal/cmd/ delegate to this package after parsing CLI input.
// This separation keeps Cobra handlers thin (parse + format) while centralizing
// business logic in a Cobra-free layer that is easy to test and reuse across
// commands (run, get, set, emit, validate, etc.).
package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-envx/envx/apps/envx/internal/manifest"
	"github.com/go-envx/envx/apps/envx/internal/merge"
	"github.com/go-envx/envx/apps/envx/internal/runner"
)

// -------------------------------------------------------------------------------------
// RunFunc is the signature for executing a child process with environment
// injection. The default is runner.Run; tests can substitute a mock.
type RunFunc func(ctx context.Context, args []string, opts runner.Options) error

// -------------------------------------------------------------------------------------
// App holds shared dependencies and provides pipeline methods. Construct via
// New() and inject into command constructors.
type App struct {
	ConfigResolver *manifest.Resolver
	Runner         RunFunc
}

// -------------------------------------------------------------------------------------
// New creates an App with production dependencies.
func New() *App {
	return &App{
		ConfigResolver: manifest.NewResolver(),
		Runner:         runner.Run,
	}
}

// -------------------------------------------------------------------------------------
// Pipeline holds the results of the shared pipeline stages. Commands access
// intermediate results (e.g. the loaded manifest) when they need them.
type Pipeline struct {
	Manifest *manifest.Manifest
	Project  manifest.ProjectMatch
}

// -------------------------------------------------------------------------------------
// LoadManifest discovers and loads the manifest file. This is the entry point
// for any command that needs workspace context.
func (a *App) LoadManifest(configPath string) (*manifest.Manifest, error) {
	manifestPath, err := manifest.Discover(configPath)
	if err != nil {
		return nil, err
	}
	return manifest.Load(manifestPath)
}

// -------------------------------------------------------------------------------------
// ResolveProject looks up a project by name or path within the manifest.
func (a *App) ResolveProject(
	m *manifest.Manifest,
	projectRef string,
) (manifest.ProjectMatch, error) {
	match, ok := m.LookupProject(projectRef)
	if !ok {
		return manifest.ProjectMatch{}, fmt.Errorf(
			"project %q not found in manifest",
			projectRef,
		)
	}
	return match, nil
}

// -------------------------------------------------------------------------------------
// ValidateEnvironment checks that the given environment is declared in the manifest.
func (a *App) ValidateEnvironment(m *manifest.Manifest, environment string) error {
	if !m.HasEnvironment(environment) {
		return fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			environment,
			m.Environments,
		)
	}
	return nil
}

// -------------------------------------------------------------------------------------
// BuildNamespaces constructs the ordered namespace list for a project. Includes
// come first (in declaration order), followed by the project's own namespace.
func (a *App) BuildNamespaces(
	m *manifest.Manifest,
	match *manifest.ProjectMatch,
) []merge.Namespace {
	proj := &match.Project
	namespaces := make([]merge.Namespace, 0, len(proj.Includes)+1)

	for _, inc := range proj.Includes {
		absDir := filepath.Join(m.Dir(), filepath.Dir(inc))
		name := filepath.Base(inc)
		namespaces = append(namespaces, merge.Namespace{Dir: absDir, Name: name})
	}

	// Project's own namespace: the project name is used as the file basename.
	namespaces = append(namespaces, merge.Namespace{
		Dir:  m.ProjectDir(proj),
		Name: match.Name,
	})

	return namespaces
}

// -------------------------------------------------------------------------------------
// MergeEnv resolves and merges environment variables for the given namespaces
// and environment, using the resolved configuration.
func (a *App) MergeEnv(
	namespaces []merge.Namespace,
	environment string,
	cfg manifest.ResolvedConfig,
) (*merge.Result, error) {
	return merge.Resolve(namespaces, environment, merge.Options{
		Strict:          cfg.Strict,
		Prefix:          cfg.Prefix,
		Suffix:          cfg.Suffix,
		NamespacePrefix: cfg.NamespacePrefix,
	})
}

// -------------------------------------------------------------------------------------
// ResolvePipeline runs the full shared pipeline: manifest → project → validate
// environment → resolve config → build namespaces → merge. Most commands will
// call this as their primary entry point.
func (a *App) ResolvePipeline(
	configPath, projectRef, environment string,
	flags manifest.RawFlags,
	changed manifest.FlagSet,
) (*Pipeline, manifest.ResolvedConfig, *merge.Result, error) {
	m, err := a.LoadManifest(configPath)
	if err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	if err := a.ValidateEnvironment(m, environment); err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	match, err := a.ResolveProject(m, projectRef)
	if err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	cfg := a.ConfigResolver.Resolve(flags, changed, m, &match.Project)
	namespaces := a.BuildNamespaces(m, &match)

	result, err := a.MergeEnv(namespaces, environment, cfg)
	if err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	pipeline := &Pipeline{
		Manifest: m,
		Project:  match,
	}

	return pipeline, cfg, result, nil
}

// -------------------------------------------------------------------------------------
// RunOptions holds the parameters for Run that are specific to process execution.
type RunOptions struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------
// Run executes the full run pipeline: resolve env + exec child process.
func (a *App) Run(
	ctx context.Context,
	configPath,
	projectRef,
	environment string,
	flags manifest.RawFlags,
	changed manifest.FlagSet,
	opts RunOptions,
) error {
	_, cfg, result, err := a.ResolvePipeline(
		configPath,
		projectRef,
		environment,
		flags,
		changed,
	)
	if err != nil {
		return err
	}

	return a.Runner(ctx, opts.Args, runner.Options{
		Env:      result.Env,
		Overload: cfg.Overload,
		Stdout:   opts.Stdout,
		Stderr:   opts.Stderr,
	})
}
