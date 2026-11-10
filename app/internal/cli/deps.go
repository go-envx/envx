package cli

import (
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/pack"
	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/validate"
)

// envFactory assembles the environment service for a manifest path.
type envFactory interface {
	NewEnvService(configPath string) (*env.Service, error)
}

// secretsFactory assembles the secrets service for a manifest path.
type secretsFactory interface {
	NewSecretsService(configPath string) (*secrets.Service, error)
}

// packFactory assembles the pack service for a manifest path.
type packFactory interface {
	NewPackService(configPath string) (*pack.Service, error)
}

// validateFactory assembles the validation service for a manifest path.
type validateFactory interface {
	NewValidateService(configPath string) (*validate.Service, error)
}

// scaffoldFactory returns the workspace-independent scaffold service.
type scaffoldFactory interface {
	NewScaffoldService() *scaffold.Service
}

// appFactory aggregates every feature factory for root command construction.
type appFactory interface {
	envFactory
	secretsFactory
	packFactory
	validateFactory
	scaffoldFactory
}
