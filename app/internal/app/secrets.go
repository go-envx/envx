package app

import (
	"sync"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/secrets/keyring"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// NewSecretsService assembles the secrets service for the workspace at configPath.
func (f Factory) NewSecretsService(configPath string) (*secrets.Service, error) {
	ws, err := loadWorkspace(configPath)
	if err != nil {
		return nil, err
	}
	return secretsService(&ws, f.hostEnvironment())
}

// secretsService wires the secrets service; the host environment is captured
// once, on the first private-key lookup.
func secretsService(
	ws *workspace.Workspace,
	hostEnv func() hostenv.Snapshot,
) (*secrets.Service, error) {
	cipher, err := newCipher(ws.Secrets.Cipher)
	if err != nil {
		return nil, err
	}
	store, err := newSecretsStore(ws)
	if err != nil {
		return nil, err
	}
	hostSnapshot := sync.OnceValue(hostEnv)
	keysPath := ws.Secrets.KeysPath
	keys, err := keyring.New(keysPath, func(name string) (string, bool) {
		return hostSnapshot().Get(name)
	})
	if err != nil {
		return nil, err
	}
	return secrets.New(store, keys, cipher)
}
