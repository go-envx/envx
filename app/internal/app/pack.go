package app

import (
	"github.com/go-envx/envx/app/internal/core/pack"
)

// NewPackService assembles the pack service for the workspace at configPath. Packing
// copies ciphertext, so no cipher or private keys are wired.
func (Factory) NewPackService(configPath string) (*pack.Service, error) {
	ws, err := loadWorkspace(configPath)
	if err != nil {
		return nil, err
	}
	secretsStore, err := newSecretsStore(&ws)
	if err != nil {
		return nil, err
	}
	return pack.New(ws, pack.Dependencies{
		Secrets: secretsStore,
	})
}
