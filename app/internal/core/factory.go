package core

import (
	"sync"

	"github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/runner"
	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/validate"
)

// AppFactory hands out services, assembling workspace-bound ones per config path.
type AppFactory struct {
	base BaseServices
	apps sync.Map // config path -> *assembly
}

// assembly memoizes one workspace's composed services or the error that prevented them.
type assembly struct {
	once     sync.Once
	services AppServices
	err      error
}

// NewAppFactory constructs the application factory and its workspace-free services.
func NewAppFactory() (*AppFactory, error) {
	base, err := composeBaseServices()
	if err != nil {
		return nil, err
	}
	return &AppFactory{base: base}, nil
}

// compose runs config, client, and service composition exactly once.
func (a *assembly) compose(configPath string) (AppServices, error) {
	a.once.Do(func() {
		a.services, a.err = composeWorkspaceServices(configPath)
	})
	return a.services, a.err
}

// composeWorkspaceServices composes config, clients, and services for one workspace.
func composeWorkspaceServices(configPath string) (AppServices, error) {
	config, err := composeAppConfig(configPath)
	if err != nil {
		return AppServices{}, err
	}

	clients, err := composeAppClients(&config)
	if err != nil {
		return AppServices{}, err
	}

	return composeAppServices(&config, clients)
}

// workspaceServices returns the memoized services for the workspace at configPath.
func (f *AppFactory) workspaceServices(configPath string) (AppServices, error) {
	entry, _ := f.apps.LoadOrStore(configPath, &assembly{})
	return entry.(*assembly).compose(configPath)
}

// ScaffoldService returns the workspace-free scaffold service.
func (f *AppFactory) ScaffoldService() (*scaffold.Service, error) {
	return f.base.Scaffold, nil
}

// RunnerService returns the workspace-free process execution service.
func (f *AppFactory) RunnerService() (*runner.Service, error) {
	return f.base.Runner, nil
}

// EmitService returns the workspace-free target serialization service.
func (f *AppFactory) EmitService() (*emit.Service, error) {
	return f.base.Emit, nil
}

// SecretsService returns the secrets service for the workspace at configPath.
func (f *AppFactory) SecretsService(configPath string) (*secrets.Service, error) {
	services, err := f.workspaceServices(configPath)
	if err != nil {
		return nil, err
	}
	return services.Secrets, nil
}

// EnvService returns the env service for the workspace at configPath.
func (f *AppFactory) EnvService(configPath string) (*env.Service, error) {
	services, err := f.workspaceServices(configPath)
	if err != nil {
		return nil, err
	}
	return services.Env, nil
}

// PackService returns the pack service for the workspace at configPath.
func (f *AppFactory) PackService(configPath string) (*pack.Service, error) {
	services, err := f.workspaceServices(configPath)
	if err != nil {
		return nil, err
	}
	return services.Pack, nil
}

// ValidateService returns the validate service for the workspace at configPath.
func (f *AppFactory) ValidateService(configPath string) (*validate.Service, error) {
	services, err := f.workspaceServices(configPath)
	if err != nil {
		return nil, err
	}
	return services.Validate, nil
}
