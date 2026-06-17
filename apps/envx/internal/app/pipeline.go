package app

import (
	"fmt"
	"path/filepath"

	"github.com/go-envx/envx/apps/envx/internal/manifest"
	"github.com/go-envx/envx/apps/envx/internal/merge"
)

// -------------------------------------------------------------------------------------
// PipelineInput holds the common inputs needed by every command's pipeline.
// This avoids passing the same 4-tuple of arguments through every method.
type PipelineInput struct {
	ConfigPath string
	ProjectRef string
	Flags      *manifest.RawFlags
	Changed    manifest.FlagSet
}

// -------------------------------------------------------------------------------------
// PipelineResult holds the results of the shared pipeline stages. Commands
// access intermediate results (e.g. the loaded manifest) when they need them.
type PipelineResult struct {
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
// BuildNamespaces constructs the ordered namespace list for a project from its
// includes. Each include is resolved to a merge.Namespace with an absolute
// directory and base name.
func (a *App) BuildNamespaces(
	m *manifest.Manifest,
	match *manifest.ProjectMatch,
) []merge.Namespace {
	proj := &match.Project
	namespaces := make([]merge.Namespace, 0, len(proj.Includes))

	for _, inc := range proj.Includes {
		absDir := filepath.Join(m.Dir(), filepath.Dir(inc))
		name := filepath.Base(inc)
		namespaces = append(namespaces, merge.Namespace{Dir: absDir, Name: name})
	}

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
// ResolvePipeline runs the full shared pipeline: manifest → project → resolve
// config → validate environment → build namespaces → merge. Most commands will
// call this as their primary entry point.
func (a *App) ResolvePipeline(
	in PipelineInput,
) (*PipelineResult, manifest.ResolvedConfig, *merge.Result, error) {
	m, err := a.LoadManifest(in.ConfigPath)
	if err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	match, err := a.ResolveProject(m, in.ProjectRef)
	if err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	cfg := a.ConfigResolver.Resolve(in.Flags, in.Changed, m, &match.Project)

	if err := a.ValidateEnvironment(m, cfg.Environment); err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	namespaces := a.BuildNamespaces(m, &match)

	result, err := a.MergeEnv(namespaces, cfg.Environment, cfg)
	if err != nil {
		return nil, manifest.ResolvedConfig{}, nil, err
	}

	pipeline := &PipelineResult{
		Manifest: m,
		Project:  match,
	}

	return pipeline, cfg, result, nil
}
