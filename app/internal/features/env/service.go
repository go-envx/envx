package env

import (
	"errors"
	"fmt"
	"slices"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
	"github.com/go-envx/envx/app/internal/shared/value"
)

// NamespaceData holds unmarshaled key-value tree data loaded for a namespace.
type NamespaceData struct {
	// Data is the raw unmarshaled key-value mapping.
	Data map[string]any
	// SourcePath is the provenance file path or identifier for this namespace data.
	SourcePath string
}

// NamespaceRepository loads raw namespace data consumed by Service.
type NamespaceRepository interface {
	// LoadBase loads the base namespace tree for an include path.
	LoadBase(includePath string) (NamespaceData, error)
	// LoadOverlay loads an environment-specific overlay tree, reporting false if
	// absent.
	LoadOverlay(includePath, env string) (NamespaceData, bool, error)
	// SetOverlay sets a key-value in an environment overlay file and returns the
	// file path.
	SetOverlay(includePath, env, key, value string) (string, error)
}

// ValueResolver dereferences one winning scalar value and returns unrecognized
// values unchanged.
type ValueResolver = value.Resolver

// SecretsService is what env consumes from secrets: a fresh, operation-scoped
// value resolver. Each resolving operation opens a new resolver after namespace
// winner selection, so no store snapshot or private-key cache survives the
// operation.
type SecretsService interface {
	// OpenResolver returns a fresh resolver materializing references under the
	// reveal policy: a revealing resolver decrypts, a masking resolver returns
	// canonical references.
	OpenResolver(reveal bool) (value.Resolver, error)
}

// ServiceParams provides dependencies to the environment domain service.
type ServiceParams struct {
	// Config is the manifest-derived workspace configuration.
	Config Config
	// Repository loads base and overlay namespace data. Required.
	Repository NamespaceRepository
	// Includes is an ordered chain of namespaces to merge when no project is
	// declared.
	Includes []string
	// SecretsService opens a fresh, operation-scoped value resolver on demand. A
	// nil service is identity behavior for callers with no reference syntax.
	SecretsService SecretsService
	// OSEnvironment is the injected snapshot of the process environment used to
	// compose the effective environment.
	OSEnvironment map[string]string
}

// Service coordinates overlay merging, key canonicalization, and variable
// substitution. It binds validated merge configuration without loading namespace
// files, constructing secrets dependencies, opening the secrets store, or
// constructing a value resolver. Each operation starts from fresh namespace and
// resolver state, so repeated calls observe filesystem changes and cannot reuse
// secret material.
type Service struct {
	// params is the normalized, privately-owned configuration copied at
	// construction so caller mutation cannot change service behavior.
	params ServiceParams
	// grammar is the compiled reference syntax, built once at construction from the
	// configured (or default) reference patterns so an invalid pattern fails here
	// rather than mid-operation.
	grammar *syntax.Grammar
}

// NewService validates structural settings, applies terminal defaults, and privately
// copies params without performing I/O. It permits a nil SecretsService, which
// remains identity behavior for callers that deliberately have no reference
// syntax, and does not require namespace or secrets files to exist; missing or
// malformed files are reported by the operation that needs them.
func NewService(params ServiceParams) (*Service, error) {
	if params.Repository == nil {
		return nil, errors.New("repository is required")
	}
	normalized, err := normalizeParams(params)
	if err != nil {
		return nil, err
	}
	grammar, err := syntax.NewGrammar(syntax.GrammarParams{
		ReferencePattern: normalized.Config.Settings.ReferencePattern,
	})
	if err != nil {
		return nil, err
	}
	return &Service{params: normalized, grammar: grammar}, nil
}

// WorkspaceDir returns the configured workspace directory.
func (s *Service) WorkspaceDir() string {
	return s.params.Config.WorkspaceDir
}

// operationContext holds resolved includes, environment, settings, and grammar
// for a single operation.
type operationContext struct {
	environment string
	includes    []string
	settings    Settings
	grammar     *syntax.Grammar
}

// resolveContext determines the effective project includes, target environment,
// settings, and grammar for an operation.
func (s *Service) resolveContext(
	project string,
	envOverride *string,
	opts Options,
	direct Settings,
) (operationContext, error) {
	projectName := project

	var includes []string
	var projOpts Options
	switch {
	case projectName != "":
		if proj, ok := s.params.Config.Projects[projectName]; ok {
			includes = proj.Includes
			projOpts = proj.Settings
		} else if len(s.params.Config.Projects) == 0 && len(s.params.Includes) > 0 {
			includes = s.params.Includes
		} else {
			return operationContext{}, fmt.Errorf(
				"%w: project %q not found in manifest",
				ErrProjectNotFound, projectName,
			)
		}
	case len(s.params.Config.Projects) == 1:
		for _, proj := range s.params.Config.Projects {
			includes = proj.Includes
			projOpts = proj.Settings
			break
		}
	default:
		includes = s.params.Includes
	}

	targetEnv := s.resolveEnvironment(envOverride, opts.Env, projOpts.Env)
	environments := s.params.Config.Environments
	if targetEnv == "" && len(environments) > 0 {
		targetEnv = environments[0]
	}
	if len(environments) > 0 && !slices.Contains(environments, targetEnv) {
		return operationContext{}, fmt.Errorf(
			"%w: %q (available: %v)",
			ErrEnvironmentNotDeclared, targetEnv, environments,
		)
	}

	settings := s.resolveSettings(opts, direct, projOpts)

	grammar := s.grammar
	if settings.ReferencePattern != s.params.Config.Settings.ReferencePattern ||
		grammar == nil {
		g, err := syntax.NewGrammar(syntax.GrammarParams{
			ReferencePattern: settings.ReferencePattern,
		})
		if err != nil {
			return operationContext{}, err
		}
		grammar = g
	}

	return operationContext{
		environment: targetEnv,
		includes:    includes,
		settings:    settings,
		grammar:     grammar,
	}, nil
}

// resolveEnvironment determines the target environment following precedence:
// explicit param > options flag > project setting > default environment > first
// declared environment.
func (s *Service) resolveEnvironment(
	explicit, optsEnv, projEnv *string,
) string {
	val := PrecedenceString(explicit, optsEnv, projEnv)
	if val != "" {
		return val
	}
	if s.params.Config.DefaultEnvironment != "" {
		return s.params.Config.DefaultEnvironment
	}
	if len(s.params.Config.Environments) > 0 {
		return s.params.Config.Environments[0]
	}
	return ""
}

// resolveSettings merges settings with precedence:
// explicit opts > direct settings > project settings > base settings. ENVX_*
// values are resolved by the CLI into opts.
func (s *Service) resolveSettings(
	opts Options, direct Settings, projOpts Options,
) Settings {
	base := s.params.Config.Settings

	var directPrefix, basePrefix *string
	if direct.Prefix != "" {
		directPrefix = &direct.Prefix
	}
	if base.Prefix != "" {
		basePrefix = &base.Prefix
	}
	prefix := PrecedenceString(
		opts.Prefix, directPrefix, projOpts.Prefix, basePrefix,
	)

	var directSuffix, baseSuffix *string
	if direct.Suffix != "" {
		directSuffix = &direct.Suffix
	}
	if base.Suffix != "" {
		baseSuffix = &base.Suffix
	}
	suffix := PrecedenceString(
		opts.Suffix, directSuffix, projOpts.Suffix, baseSuffix,
	)

	var directDelimiter, baseDelimiter *string
	if direct.Delimiter != "" {
		directDelimiter = &direct.Delimiter
	}
	if base.Delimiter != "" {
		baseDelimiter = &base.Delimiter
	}
	delimiter := PrecedenceString(
		opts.Delimiter, directDelimiter, projOpts.Delimiter,
		baseDelimiter,
	)
	if delimiter == "" {
		delimiter = defaultDelimiter
	}

	var directPattern, basePattern *string
	if direct.ReferencePattern != "" {
		directPattern = &direct.ReferencePattern
	}
	if base.ReferencePattern != "" {
		basePattern = &base.ReferencePattern
	}
	refPattern := PrecedenceString(
		opts.ReferencePattern, directPattern,
		projOpts.ReferencePattern, basePattern,
	)

	var directOverlays, baseOverlays *bool
	if direct.RequireOverlays {
		directOverlays = &direct.RequireOverlays
	}
	if base.RequireOverlays {
		baseOverlays = &base.RequireOverlays
	}
	requireOverlays := PrecedenceBool(
		opts.RequireOverlays, directOverlays,
		projOpts.RequireOverlays, baseOverlays,
	)

	var directOverload, baseOverload *bool
	if direct.Overload {
		directOverload = &direct.Overload
	}
	if base.Overload {
		baseOverload = &base.Overload
	}
	overload := PrecedenceBool(
		opts.Overload, directOverload, projOpts.Overload,
		baseOverload,
	)

	return Settings{
		RequireOverlays:  requireOverlays,
		Prefix:           prefix,
		Suffix:           suffix,
		Delimiter:        delimiter,
		Overload:         overload,
		ReferencePattern: refPattern,
	}
}

// normalizeEnvironment applies the configured default when the call is empty,
// then the first declared environment when both are empty, and finally validates
// the environment the operation will actually use. Validating per operation lets
// an explicit environment supersede an irrelevant configured default.
func (s *Service) normalizeEnvironment(environment string) (string, error) {
	environments := s.params.Config.Environments
	if environment == "" {
		environment = s.params.Config.DefaultEnvironment
	}
	if environment == "" && len(environments) > 0 {
		environment = environments[0]
	}
	if !slices.Contains(environments, environment) {
		return "", fmt.Errorf(
			"%w: %q (available: %v)",
			ErrEnvironmentNotDeclared, environment, environments,
		)
	}
	return environment, nil
}

// openResolver obtains a fresh, operation-scoped value resolver under the reveal
// policy. A nil service yields a nil resolver, which the resolution helpers treat
// as identity behavior for callers with no reference syntax.
func (s *Service) openResolver(reveal bool) (ValueResolver, error) {
	if s.params.SecretsService == nil {
		return nil, nil
	}
	return s.params.SecretsService.OpenResolver(reveal)
}
