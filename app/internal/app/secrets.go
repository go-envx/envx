package app

import (
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/secrets/keyring"
)

// NewSecretsService assembles the secrets service for the workspace at configPath.
func (f Factory) NewSecretsService(configPath string) (*secrets.Service, error) {
	o, err := f.loadWorkspaceOptions(configPath)
	if err != nil {
		return nil, err
	}
	return secretsService(o)
}

// secretsService wires the secrets service over the shared host lookup.
func secretsService(o workspaceOptions) (*secrets.Service, error) {
	cipher, err := newCipher(o.workspace.Secrets.Cipher)
	if err != nil {
		return nil, err
	}
	store, err := newSecretsStore(&o.workspace)
	if err != nil {
		return nil, err
	}
	keys, err := keyring.New(o.workspace.Secrets.KeysPath, o.hostEnv)
	if err != nil {
		return nil, err
	}
	return secrets.New(store, keys, cipher)
}
