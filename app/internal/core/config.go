package core

import (
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/features/workspace"
	wsfilestore "github.com/go-envx/envx/app/internal/features/workspace/filestore"
)

// resolvedWorkspace holds each feature's config slice derived from one loaded
// manifest.
type resolvedWorkspace struct {
	env      env.Config
	pack     pack.Config
	secrets  secrets.Config
	validate validate.Config
}

// resolveWorkspace loads the manifest at configPath (empty walks up from the
// working directory) and derives each feature's config.
func resolveWorkspace(configPath string) (*resolvedWorkspace, error) {
	repo, err := wsfilestore.New(wsfilestore.Params{Path: configPath})
	if err != nil {
		return nil, err
	}

	wsService, err := workspace.NewService(workspace.ServiceParams{
		Repository: repo,
	})
	if err != nil {
		return nil, err
	}
	ws, err := wsService.Load()
	if err != nil {
		return nil, err
	}

	envConfig, err := env.LoadConfig(ws)
	if err != nil {
		return nil, err
	}
	packConfig, err := pack.LoadConfig(ws)
	if err != nil {
		return nil, err
	}
	secretsConfig, err := secrets.LoadConfig(ws)
	if err != nil {
		return nil, err
	}
	validateConfig, err := validate.LoadConfig(ws)
	if err != nil {
		return nil, err
	}

	return &resolvedWorkspace{
		env:      envConfig,
		pack:     packConfig,
		secrets:  secretsConfig,
		validate: validateConfig,
	}, nil
}
