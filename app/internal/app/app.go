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

// Factory assembles feature services on demand. The zero value captures the
// process environment.
type Factory struct {
	// Environment captures the host environment; nil uses hostenv.Capture.
	Environment func() hostenv.Snapshot
}

// hostEnvironment returns the host environment source.
func (f Factory) hostEnvironment() func() hostenv.Snapshot {
	if f.Environment == nil {
		return hostenv.Capture
	}
	return f.Environment
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
	configs workspace.ProvidersConfig, host hostenv.Snapshot,
) []external.Registration {
	var registrations []external.Registration
	if config := configs.OnePassword; config != nil {
		registrations = append(registrations, external.Registration{
			Provider: onepassword.New(host, onepassword.Options{}),
			Timeout:  config.Timeout,
		})
	}
	if config := configs.Bitwarden; config != nil {
		registrations = append(registrations, external.Registration{
			Provider: bitwarden.New(host, bitwarden.Options{
				Projects: config.Projects,
			}),
			Timeout: config.Timeout,
		})
	}
	if config := configs.Passwordstate; config != nil {
		registrations = append(registrations, external.Registration{
			Provider: passwordstate.New(host, passwordstate.Options{
				URL:       config.URL,
				APIKeyEnv: config.APIKeyEnv,
				ListIDs:   config.ListIDs,
				Field:     config.Field,
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
