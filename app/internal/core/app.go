package core

import (
	"github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/runner"
	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/validate"
)

// App is the application-level factory created once at process startup.
// It lazily assembles scoped domain services per execution.
type App struct{}

// NewApp constructs an App factory.
func NewApp() *App {
	return &App{}
}

// ScaffoldService returns the domain scaffold service.
func (a *App) ScaffoldService() (*scaffold.Service, error) {
	return scaffold.NewService(scaffold.ServiceParams{
		Source: scaffold.TemplatesFS,
	})
}

// RunnerService returns the domain process execution service.
func (a *App) RunnerService() (*runner.Service, error) {
	return runner.NewService(runner.ServiceParams{})
}

// EmitService returns the domain target serialization service.
func (a *App) EmitService() (*emit.Service, error) {
	return emit.NewService(emit.ServiceParams{})
}

// SecretsService resolves the workspace and returns the domain secrets service.
func (a *App) SecretsService(configPath string) (*secrets.Service, error) {
	res, err := resolveWorkspace(configPath)
	if err != nil {
		return nil, err
	}
	return newConfiguredSecretsService(res.secrets)
}

// EnvService resolves workspace configuration and returns the domain env
// service.
func (a *App) EnvService(configPath string) (*env.Service, error) {
	res, err := resolveWorkspace(configPath)
	if err != nil {
		return nil, err
	}
	return newWorkspaceEnvService(res)
}

// PackService resolves the workspace and returns the domain bundling service.
func (a *App) PackService(configPath string) (*pack.Service, error) {
	res, err := resolveWorkspace(configPath)
	if err != nil {
		return nil, err
	}
	return newPackService(res.pack)
}

// ValidateService resolves workspace configuration and returns the domain
// workspace diagnostics service.
func (a *App) ValidateService(configPath string) (*validate.Service, error) {
	res, err := resolveWorkspace(configPath)
	if err != nil {
		return nil, err
	}
	return newValidateService(res)
}
