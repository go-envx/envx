package core

import (
	"io"

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
	return runner.NewService(), nil
}

// EmitService returns the domain target serialization service writing to writer.
func (a *App) EmitService(writer io.Writer) (*emit.Service, error) {
	return emit.NewService(emit.ServiceParams{Writer: writer}), nil
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

// PackService resolves the workspace layout and returns the domain bundling
// service.
func (a *App) PackService(configPath string) (*pack.Service, error) {
	var in *Input
	if configPath != "" {
		in = &Input{ConfigPath: &configPath}
	} else {
		in = &Input{}
	}
	layout, err := ResolveWorkspaceLayout(in)
	if err != nil {
		return nil, err
	}
	return NewPackService(layout)
}

// ValidateService resolves workspace configuration and returns the domain
// workspace diagnostics service.
func (a *App) ValidateService(configPath string) (*validate.Service, error) {
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
	return NewValidateService(res)
}
