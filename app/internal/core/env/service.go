package env

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/core/env/syntax"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// defaultDelimiter joins list items when no delimiter is configured.
const defaultDelimiter = ","

// Service runs environment workflows over a privately copied workspace
// configuration. Each operation reads fresh namespace data, captures the host
// environment once, and opens its own resolver.
type Service struct {
	// root is the workspace root directory.
	root string
	// environments lists the declared environments in order.
	environments []string
	// projects maps declared project names to their definitions.
	projects map[string]workspace.Project
	// settings holds the workspace-level settings.
	settings workspace.Settings
	// grammar is the compiled workspace reference syntax.
	grammar *syntax.Grammar
	// deps holds the adapters.
	deps Dependencies
}

// operation is the resolved context of one workflow call.
type operation struct {
	// project is the selected project name.
	project string
	// environment is the selected environment.
	environment string
	// includes are the project's absolute include paths.
	includes []string
	// settings are the precedence-resolved settings.
	settings resolvedSettings
	// grammar is the reference syntax for the resolved pattern.
	grammar *syntax.Grammar
	// host is the host environment captured for this operation.
	host hostenv.Snapshot
}

// resolvedSettings are the effective settings after precedence.
type resolvedSettings struct {
	// prefix is prepended to every key.
	prefix string
	// suffix is appended to every key.
	suffix string
	// delimiter joins list items.
	delimiter string
	// referencePattern is the reference syntax; empty is the default.
	referencePattern string
	// overload lets namespace values win over host values.
	overload bool
	// requireOverlays requires every namespace overlay to exist.
	requireOverlays bool
}

// New validates structural settings and privately copies the workspace
// configuration without performing I/O. Only EnvNamespaces is required.
func New(ws workspace.Workspace, deps Dependencies) (*Service, error) {
	if deps.EnvNamespaces == nil {
		return nil, errors.New("namespace reader is required")
	}
	grammar, err := syntax.Compile(deref(ws.Settings.ReferencePattern))
	if err != nil {
		return nil, err
	}
	projects := make(map[string]workspace.Project, len(ws.Projects))
	for name, p := range ws.Projects {
		if _, err := syntax.Compile(deref(p.Settings.ReferencePattern)); err != nil {
			return nil, fmt.Errorf("project %q: %w", name, err)
		}
		projects[name] = workspace.Project{
			Includes:     slices.Clone(p.Includes),
			IncludePaths: slices.Clone(p.IncludePaths),
			Settings:     cloneSettings(p.Settings),
		}
	}
	return &Service{
		root:         ws.Root,
		environments: slices.Clone(ws.Environments),
		projects:     projects,
		settings:     cloneSettings(ws.Settings),
		grammar:      grammar,
		deps:         deps,
	}, nil
}

// begin resolves the project, environment, settings, and grammar for a query
// and captures the host environment.
func (s *Service) begin(query Query) (operation, error) {
	name, project, err := s.project(query.Project)
	if err != nil {
		return operation{}, err
	}
	environment, err := s.environment(query.Options.Env, project.Settings.Env)
	if err != nil {
		return operation{}, err
	}
	settings := s.resolveSettings(query.Options, project.Settings)
	grammar := s.grammar
	if settings.referencePattern != deref(s.settings.ReferencePattern) {
		if grammar, err = syntax.Compile(settings.referencePattern); err != nil {
			return operation{}, err
		}
	}
	return operation{
		project:     name,
		environment: environment,
		includes:    project.IncludePaths,
		settings:    settings,
		grammar:     grammar,
		host:        s.host(),
	}, nil
}

// load reads the operation's namespaces and merges one environment, applying
// host source selection; union adds host-only keys.
func (s *Service) load(op operation, union bool) (*mergedEnv, error) {
	namespaces, err := s.loadNamespaces(op.includes)
	if err != nil {
		return nil, err
	}
	merged, err := s.merge(namespaces, op)
	if err != nil {
		return nil, err
	}
	merged.applyHost(op, union)
	return merged, nil
}

// project selects a declared project; an empty name selects the only one.
func (s *Service) project(name string) (string, workspace.Project, error) {
	if name == "" {
		if len(s.projects) == 1 {
			for only, p := range s.projects {
				return only, p, nil
			}
		}
		return "", workspace.Project{}, fmt.Errorf(
			"%w: no project selected (declared: %s)",
			ErrProjectNotFound, strings.Join(slices.Sorted(maps.Keys(s.projects)), ", "),
		)
	}
	p, ok := s.projects[name]
	if !ok {
		return "", workspace.Project{}, fmt.Errorf("%w: %q", ErrProjectNotFound, name)
	}
	return name, p, nil
}

// environment selects the first non-empty candidate, then the workspace
// setting, then the first declared environment, and verifies it is declared.
func (s *Service) environment(candidates ...*string) (string, error) {
	candidates = append(candidates, s.settings.Env)
	selected := firstString(nil, candidates...)
	if selected == "" && len(s.environments) > 0 {
		selected = s.environments[0]
	}
	if !slices.Contains(s.environments, selected) {
		return "", fmt.Errorf(
			"%w: %q (available: %v)", ErrEnvironmentNotDeclared, selected, s.environments,
		)
	}
	return selected, nil
}

// host captures the host environment; a nil source is empty.
func (s *Service) host() hostenv.Snapshot {
	if s.deps.HostEnv == nil {
		return hostenv.Snapshot{}
	}
	return s.deps.HostEnv()
}

// newValueResolver creates an operation-scoped resolver bound to host. A nil
// factory yields a nil resolver, which resolution treats as identity.
func (s *Service) newValueResolver(
	reveal bool, host hostenv.Snapshot,
) (ValueResolver, error) {
	if s.deps.NewValueResolver == nil {
		return nil, nil
	}
	return s.deps.NewValueResolver(reveal, host)
}

// resolveSettings applies precedence: overrides, project, then workspace.
func (s *Service) resolveSettings(
	overrides, project workspace.Settings,
) resolvedSettings {
	global := s.settings
	delimiter := firstString(overrides.Delimiter, project.Delimiter, global.Delimiter)
	if delimiter == "" {
		delimiter = defaultDelimiter
	}
	return resolvedSettings{
		prefix:    firstString(overrides.Prefix, project.Prefix, global.Prefix),
		suffix:    firstString(overrides.Suffix, project.Suffix, global.Suffix),
		delimiter: delimiter,
		referencePattern: firstString(
			overrides.ReferencePattern, project.ReferencePattern, global.ReferencePattern,
		),
		overload: firstBool(overrides.Overload, project.Overload, global.Overload),
		requireOverlays: firstBool(
			overrides.RequireOverlays, project.RequireOverlays, global.RequireOverlays,
		),
	}
}

// firstString returns explicit when set, even if empty, then the first
// non-empty layer, then "".
func firstString(explicit *string, layers ...*string) string {
	if explicit != nil {
		return *explicit
	}
	for _, layer := range layers {
		if layer != nil && *layer != "" {
			return *layer
		}
	}
	return ""
}

// firstBool returns the first set value, then false.
func firstBool(layers ...*bool) bool {
	for _, layer := range layers {
		if layer != nil {
			return *layer
		}
	}
	return false
}

// deref returns the pointed-to string or "".
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// cloneSettings copies every set value so later caller mutation is not shared.
func cloneSettings(in workspace.Settings) workspace.Settings {
	return workspace.Settings{
		Delimiter:        clonePtr(in.Delimiter),
		Env:              clonePtr(in.Env),
		Overload:         clonePtr(in.Overload),
		Prefix:           clonePtr(in.Prefix),
		ReferencePattern: clonePtr(in.ReferencePattern),
		RequireOverlays:  clonePtr(in.RequireOverlays),
		Suffix:           clonePtr(in.Suffix),
	}
}

// clonePtr copies the pointed-to value.
func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
