package app

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/core/external/bitwarden"
	"github.com/go-envx/envx/app/internal/core/external/onepassword"
	"github.com/go-envx/envx/app/internal/core/external/passwordstate"
	"github.com/go-envx/envx/app/internal/core/secrets"
	secretsStore "github.com/go-envx/envx/app/internal/core/secrets/filestore"
	"github.com/go-envx/envx/app/internal/core/workspace"
	workspaceStore "github.com/go-envx/envx/app/internal/core/workspace/filestore"
	"github.com/go-envx/envx/app/internal/lib/crypt/age"
	"github.com/go-envx/envx/app/internal/lib/crypt/naclbox"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// Factory assembles feature services on demand. The zero value reads the
// process environment.
type Factory struct {
	// Environ returns the host environment as KEY=value entries; nil uses
	// os.Environ.
	Environ func() []string
}

// workspaceOptions is a loaded workspace with the host lookup its services
// share.
type workspaceOptions struct {
	workspace workspace.Workspace
	hostEnv   *hostenv.Lookup
}

// loadWorkspaceOptions loads the workspace at configPath and pairs it with the
// process environment layered over its local env file.
func (f Factory) loadWorkspaceOptions(configPath string) (workspaceOptions, error) {
	ws, err := loadWorkspace(configPath)
	if err != nil {
		return workspaceOptions{}, err
	}
	he := hostenv.NewLookup(hostenv.Options{
		File:    ws.LocalEnvPath,
		Environ: f.Environ,
	})
	return workspaceOptions{
		workspace: ws,
		hostEnv:   he,
	}, nil
}

// loadWorkspace loads the manifest at configPath, discovering it from the
// working directory when empty.
func loadWorkspace(configPath string) (workspace.Workspace, error) {
	repository, err := workspaceStore.New(
		workspaceStore.Params{Path: configPath},
	)
	if err != nil {
		return workspace.Workspace{}, err
	}
	ws, err := repository.Load()
	if err != nil {
		return workspace.Workspace{}, err
	}
	return *ws, nil
}

// newCipher selects the engine for the configured algorithm.
func newCipher(algorithm string) (secrets.Cipher, error) {
	switch algorithm {
	case age.Algorithm:
		return age.Cipher{}, nil
	case naclbox.Algorithm:
		return naclbox.Cipher{}, nil
	default:
		return nil, fmt.Errorf("unsupported cipher algorithm: %q", algorithm)
	}
}

// newProviders registers an adapter for each declared provider.
func newProviders(
	configs workspace.ProvidersConfig, hostEnv *hostenv.Lookup,
) []external.Registration {
	var registrations []external.Registration
	if config := configs.OnePassword; config != nil {
		registrations = append(registrations, external.Registration{
			Provider: onepassword.New(hostEnv, onepassword.Options{}),
			Timeout:  config.Timeout,
		})
	}
	if config := configs.Bitwarden; config != nil {
		registrations = append(registrations, external.Registration{
			Provider: bitwarden.New(hostEnv, bitwarden.Options{
				Projects: config.Projects,
			}),
			Timeout: config.Timeout,
		})
	}
	if config := configs.Passwordstate; config != nil {
		registrations = append(registrations, external.Registration{
			Provider: passwordstate.New(hostEnv, passwordstate.Options{
				URL:     config.URL,
				ListIDs: config.ListIDs,
				Field:   config.Field,
			}),
			Timeout: config.Timeout,
		})
	}
	return registrations
}

// newSecretsStore constructs the workspace secrets document adapter.
func newSecretsStore(ws *workspace.Workspace) (*secretsStore.Repository, error) {
	return secretsStore.New(
		ws.Secrets.SecretsPath,
		secretsStore.Options{Indent: ws.Indent},
	)
}
