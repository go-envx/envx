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

// Dependencies open the inspection adapters lazily; both are required.
type Dependencies struct {
	// OpenEnv opens the explainer when a selected check reads the merge.
	OpenEnv func() (Explainer, error)
	// OpenSecrets opens the inspector when a selected check reads the store. It
	// must resolve private keys as the explainer's resolver does.
	OpenSecrets func() (Inspector, error)
}
