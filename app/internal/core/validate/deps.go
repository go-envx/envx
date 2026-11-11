package validate

import (
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/secrets"
)

// Explainer diagnoses one project environment without aborting per value.
type Explainer interface {
	// Explain diagnoses the selected winners of one project environment.
	Explain(params env.ExplainParams) (env.ExplainResult, error)
}

// Inspector audits the secrets store without decrypting values.
type Inspector interface {
	// Audit reports store health for the selected sections.
	Audit(scope secrets.AuditScope) (secrets.Audit, error)
}

// ProviderInspector checks declared external providers without contacting them.
type ProviderInspector interface {
	// CheckAvailable returns why the declared provider for scheme cannot be
	// called; nil when it can or is not declared.
	CheckAvailable(scheme string) error
}

// Dependencies open the inspection adapters lazily; all are required.
type Dependencies struct {
	// OpenEnv opens the explainer when a selected check reads the merge.
	OpenEnv func() (Explainer, error)
	// OpenSecrets opens the inspector when a selected check reads the store. It
	// must resolve private keys as the explainer's resolver does.
	OpenSecrets func() (Inspector, error)
	// OpenProviders opens the provider inspector when the provider check runs
	// and some value references an external provider.
	OpenProviders func() (ProviderInspector, error)
}
