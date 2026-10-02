package validate

import (
	"errors"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// EnvironmentDiagnoser defines what validate consumes from environment
// resolution: a masked, non-aborting explanation of one project environment.
type EnvironmentDiagnoser interface {
	// Explain diagnoses every winning value of one project environment.
	Explain(params env.ExplainParams) (*env.ExplainResult, error)
}

// StoreDiagnoser defines what validate consumes from secrets store inspection:
// offline views of the stored values and group keypairs that never expose
// plaintext or private-key material.
type StoreDiagnoser interface {
	// StoredSecrets lists every stored entry with how its value is encoded.
	StoredSecrets() ([]secrets.StoredSecret, error)
	// GroupsMissingPublicKey lists groups with stored secrets but no public key.
	GroupsMissingPublicKey() ([]string, error)
	// ListKeypairs reports every group keypair's private-key status.
	ListKeypairs() ([]secrets.KeypairMetadata, error)
}

// ServiceParams provides dependencies to the workspace diagnostics service.
type ServiceParams struct {
	// Environment diagnoses each project in each declared environment.
	Environment EnvironmentDiagnoser
	// Store provides the store-level findings; nil skips them, running only the
	// per-environment resolution checks.
	Store StoreDiagnoser
	// Projects lists every project to diagnose.
	Projects []string
	// Environments lists the declared environments every project is diagnosed
	// against.
	Environments []string
	// Severity overrides the default reporting level per status code, keyed by
	// canonical code. A nil map leaves every code at its default; a code mapped to
	// status.Off suppresses its findings entirely.
	Severity map[string]status.Severity
}

// Service diagnoses a whole workspace for resolution and store problems.
type Service struct {
	// params is the privately-owned configuration copied at construction.
	params ServiceParams
}

// NewService constructs a workspace diagnostics domain service.
func NewService(params ServiceParams) (*Service, error) {
	if params.Environment == nil {
		return nil, errors.New("environment diagnoser is required")
	}
	return &Service{params: params}, nil
}
