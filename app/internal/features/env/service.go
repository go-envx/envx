package env

import (
	"errors"
	"fmt"
	"slices"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
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
// values unchanged. env is the active environment, used by implementations that
// support environment-implicit references.
type ValueResolver interface {
	// Resolve returns value with any recognized reference dereferenced, or value
	// unchanged when it is not a reference.
	Resolve(value, env string) (string, error)
}

// ValueResolverFactory opens a fresh, operation-scoped value resolver under the
// requested reveal policy. Each resolving operation asks for a new resolver after
// namespace winner selection, so no store snapshot or private-key cache survives
// the operation.
type ValueResolverFactory interface {
	// Resolver returns a fresh resolver materializing references under the reveal
	// policy: a revealing resolver decrypts, a masking resolver returns canonical
	// references.
	Resolver(reveal bool) (ValueResolver, error)
}

// ServiceParams provides dependencies to the environment domain service.
type ServiceParams struct {
	// WorkspaceDir is the workspace root directory.
	WorkspaceDir string
	// Repository loads base and overlay namespace data. Required.
	Repository NamespaceRepository
	// Includes is an ordered chain of namespaces to merge, given as include paths
	// the caller has already resolved.
	Includes []string
	// Environments lists the declared environments, used to validate the target.
	Environments []string
	// DefaultEnvironment is the precedence-resolved default an operation uses when
	// it is given no explicit environment.
	DefaultEnvironment string
	// Settings holds the fully-resolved env-resolution knobs the merge reads.
	Settings Settings
	// ResolverFactory opens a fresh, operation-scoped value resolver on demand. A
	// nil factory is identity behavior for callers with no reference syntax.
	ResolverFactory ValueResolverFactory
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
// copies params without performing I/O. It permits a nil ResolverFactory, which
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
		ReferencePattern: normalized.Settings.ReferencePattern,
	})
	if err != nil {
		return nil, err
	}
	return &Service{params: normalized, grammar: grammar}, nil
}

// WorkspaceDir returns the configured workspace directory.
func (s *Service) WorkspaceDir() string {
	return s.params.WorkspaceDir
}

// normalizeEnvironment applies the configured default when the call is empty,
// then the first declared environment when both are empty, and finally validates
// the environment the operation will actually use. Validating per operation lets
// an explicit environment supersede an irrelevant configured default.
func (s *Service) normalizeEnvironment(environment string) (string, error) {
	if environment == "" {
		environment = s.params.DefaultEnvironment
	}
	if environment == "" && len(s.params.Environments) > 0 {
		environment = s.params.Environments[0]
	}
	if !slices.Contains(s.params.Environments, environment) {
		return "", fmt.Errorf(
			"%w: %q (available: %v)",
			ErrEnvironmentNotDeclared, environment, s.params.Environments,
		)
	}
	return environment, nil
}

// openResolver obtains a fresh, operation-scoped value resolver under the reveal
// policy. A nil factory yields a nil resolver, which the resolution helpers treat
// as identity behavior for callers with no reference syntax.
func (s *Service) openResolver(reveal bool) (ValueResolver, error) {
	if s.params.ResolverFactory == nil {
		return nil, nil
	}
	return s.params.ResolverFactory.Resolver(reveal)
}
