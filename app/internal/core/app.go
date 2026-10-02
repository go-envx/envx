package core

import (
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/features/secrets"
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

// SecretsService resolves the workspace and returns the domain secrets service.
func (a *App) SecretsService(configPath string) (*secrets.Service, error) {
	var in *Input
	if configPath != "" {
		in = &Input{ConfigPath: &configPath}
	} else {
		in = &Input{}
	}
	c, err := ResolveWorkspace(in)
	if err != nil {
		return nil, err
	}
	return NewSecretsManager(c.Secrets, c.Cipher)
}

// EnvService resolves workspace configuration and returns the domain env
// service.
func (a *App) EnvService(configPath string) (*env.Service, error) {
	var in *Input
	if configPath != "" {
		in = &Input{ConfigPath: &configPath}
	} else {
		in = &Input{}
	}
	res, err := ResolveWorkspace(in)
	if err != nil {
		return nil, err
	}
	return NewWorkspaceEnvService(res)
}
