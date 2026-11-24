package app

import (
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
	return secretsService(&ws, f.newHostEnv())
}

// secretsService wires the secrets service over a shared host lookup.
func secretsService(
	ws *workspace.Workspace,
	hostEnv *hostenv.Lookup,
) (*secrets.Service, error) {
	cipher, err := newCipher(ws.Secrets.Cipher)
	if err != nil {
		return nil, err
	}
	store, err := newSecretsStore(ws)
	if err != nil {
		return nil, err
	}
	keys, err := keyring.New(ws.Secrets.KeysPath, hostEnv)
	if err != nil {
		return nil, err
	}
	return secrets.New(store, keys, cipher)
}
