package core

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/features/workspace"
	wsfilestore "github.com/go-envx/envx/app/internal/features/workspace/filestore"
)

// AppConfig holds every config slice derived from the workspace manifest.
type AppConfig struct {
	Env      env.Config
	Pack     pack.Config
	Secrets  secrets.Config
	Validate validate.Config
}

// composeAppConfig loads the workspace at configPath and derives each config slice.
func composeAppConfig(configPath string) (AppConfig, error) {
	wsRepo, err := wsfilestore.New(wsfilestore.Params{Path: configPath})
	if err != nil {
		return AppConfig{}, fmt.Errorf("composing workspace repository: %w", err)
	}

	wsService, err := workspace.NewService(workspace.ServiceParams{Repository: wsRepo})
	if err != nil {
		return AppConfig{}, fmt.Errorf("composing workspace service: %w", err)
	}

	ws, err := wsService.Load()
	if err != nil {
		return AppConfig{}, err
	}

	envConfig, err := env.LoadConfig(ws)
	if err != nil {
		return AppConfig{}, err
	}

	packConfig, err := pack.LoadConfig(ws)
	if err != nil {
		return AppConfig{}, err
	}

	secretsConfig, err := secrets.LoadConfig(ws)
	if err != nil {
		return AppConfig{}, err
	}

	validateConfig, err := validate.LoadConfig(ws)
	if err != nil {
		return AppConfig{}, err
	}

	return AppConfig{
		Env:      envConfig,
		Pack:     packConfig,
		Secrets:  secretsConfig,
		Validate: validateConfig,
	}, nil
}
