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
	"github.com/go-envx/envx/app/internal/lib/process"
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

// newProviders selects the adapter for each declared provider. Providers
// without an adapter stay unregistered, so their references pass through.
func newProviders(
	configs map[string]workspace.ProviderConfig, host hostenv.Snapshot,
) []external.Registration {
	var registrations []external.Registration
	for scheme, config := range configs {
		var provider external.Provider
		switch scheme {
		case external.SchemeOnePassword:
			provider = onepassword.New(process.Output, host)
		case external.SchemeBitwarden:
			provider = bitwarden.New(process.Output, host)
		case external.SchemePasswordstate:
			provider = passwordstate.New(host, passwordstate.Options{
				URL:       config.URL,
				APIKeyEnv: config.APIKeyEnv,
			})
		default:
			continue
		}
		registrations = append(registrations, external.Registration{
			Provider: provider,
			Timeout:  config.Timeout,
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
