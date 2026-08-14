package envmerge

import (
	"fmt"
	"slices"
)

// Manager binds validated merge configuration without loading namespace files,
// constructing secrets dependencies, opening the secrets store, or constructing a
// value resolver. Each operation starts from fresh namespace and resolver state,
// so repeated calls observe filesystem changes and cannot reuse secret material.
type Manager struct {
	// params is the normalized, privately-owned configuration copied at
	// construction so caller mutation cannot change manager behavior.
	params Params
}

// New validates structural settings, applies terminal defaults, and privately
// copies params without performing I/O. It permits a nil ResolverFactory, which
// remains identity behavior for callers that deliberately have no reference
// syntax, and does not require namespace or secrets files to exist; missing or
// malformed files are reported by the operation that needs them.
func New(params Params) (*Manager, error) {
	normalized, err := normalizeParams(params)
	if err != nil {
		return nil, err
	}
	return &Manager{params: normalized}, nil
}

// normalizeEnvironment applies the configured default when the call is empty,
// then the first declared environment when both are empty, and finally validates
// the environment the operation will actually use. Validating per operation lets
// an explicit environment supersede an irrelevant configured default.
func (m *Manager) normalizeEnvironment(environment string) (string, error) {
	if environment == "" {
		environment = m.params.DefaultEnvironment
	}
	if environment == "" && len(m.params.Environments) > 0 {
		environment = m.params.Environments[0]
	}
	if !slices.Contains(m.params.Environments, environment) {
		return "", fmt.Errorf(
			"environment %q is not declared (available: %v)",
			environment, m.params.Environments,
		)
	}
	return environment, nil
}

// openResolver obtains a fresh, operation-scoped value resolver under the reveal
// policy. A nil factory yields a nil resolver, which the resolution helpers treat
// as identity behavior for callers with no reference syntax.
func (m *Manager) openResolver(reveal bool) (ValueResolver, error) {
	if m.params.ResolverFactory == nil {
		return nil, nil
	}
	return m.params.ResolverFactory.Resolver(reveal)
}
