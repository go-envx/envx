package core

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/features/runner"
	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/validate"
)

// BaseServices holds services that never require a workspace.
type BaseServices struct {
	Emit     *emit.Service
	Runner   *runner.Service
	Scaffold *scaffold.Service
}

// composeBaseServices constructs the workspace-free services.
func composeBaseServices() (BaseServices, error) {
	emitService, err := emit.NewService(emit.ServiceParams{})
	if err != nil {
		return BaseServices{}, fmt.Errorf("composing emit service: %w", err)
	}

	runnerService, err := runner.NewService(runner.ServiceParams{})
	if err != nil {
		return BaseServices{}, fmt.Errorf("composing runner service: %w", err)
	}

	scaffoldService, err := scaffold.NewService(scaffold.ServiceParams{
		Source: scaffold.TemplatesFS,
	})
	if err != nil {
		return BaseServices{}, fmt.Errorf("composing scaffold service: %w", err)
	}

	return BaseServices{
		Emit:     emitService,
		Runner:   runnerService,
		Scaffold: scaffoldService,
	}, nil
}

// AppServices holds every workspace-bound domain service.
type AppServices struct {
	PrivateKey *privatekey.Service
	Secrets    *secrets.Service
	Env        *env.Service
	Validate   *validate.Service
	Pack       *pack.Service
}

// composeAppServices constructs each service in dependency order.
func composeAppServices(config *AppConfig, clients AppClients) (AppServices, error) {
	privateKeyService, err := privatekey.NewService(privatekey.ServiceParams{
		Repository:  clients.PrivateKeyStore,
		Environment: clients.HostEnv,
	})
	if err != nil {
		return AppServices{}, fmt.Errorf("composing private key service: %w", err)
	}

	secretsService, err := secrets.NewService(secrets.ServiceParams{
		Repository:        clients.SecretsStore,
		Cipher:            clients.Cipher,
		PrivateKeyService: privateKeyService,
	})
	if err != nil {
		return AppServices{}, fmt.Errorf("composing secrets service: %w", err)
	}

	envService, err := env.NewService(env.ServiceParams{
		Config:          config.Env,
		Repository:      clients.NamespaceStore,
		HostEnvironment: clients.HostEnv,
		SecretsService:  secretsService,
	})
	if err != nil {
		return AppServices{}, fmt.Errorf("composing env service: %w", err)
	}

	validateService, err := validate.NewService(validate.ServiceParams{
		Config:         config.Validate,
		EnvService:     envService,
		SecretsService: secretsService,
	})
	if err != nil {
		return AppServices{}, fmt.Errorf("composing validate service: %w", err)
	}

	packService, err := pack.NewService(pack.ServiceParams{
		Config:          config.Pack,
		SecretsReader:   clients.SecretsStore,
		SecretsExporter: clients.SecretsExporter,
	})
	if err != nil {
		return AppServices{}, fmt.Errorf("composing pack service: %w", err)
	}

	return AppServices{
		PrivateKey: privateKeyService,
		Secrets:    secretsService,
		Env:        envService,
		Validate:   validateService,
		Pack:       packService,
	}, nil
}
