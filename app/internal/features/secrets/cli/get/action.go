package get

import (
	"github.com/go-envx/envx/app/internal/config"
)

// actionParams identifies the one secret being read.
type actionParams struct {
	// Group identifies the case-insensitive key group.
	Group string
	// Key identifies the case-sensitive secret entry.
	Key string
}

// actionResult carries the decrypted plaintext to the renderer.
type actionResult struct {
	// Value is the decrypted plaintext of the requested secret.
	Value string
}

// execute decrypts and returns one secret value. Naming a specific group and key
// is the deliberate act of retrieving a secret, so the plaintext is returned
// directly; an unavailable private key fails the operation.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// Resolve the workspace configuration and encryption settings.
	c, err := config.ResolveWorkspace(in)
	if err != nil {
		return actionResult{}, err
	}
	// Create the manager responsible for encrypted secret storage.
	manager, err := config.NewSecretsManager(c.Secrets, c.Cipher)
	if err != nil {
		return actionResult{}, err
	}

	// Decrypt the requested secret into transient plaintext.
	value, err := manager.Get(p.Group, p.Key)
	if err != nil {
		return actionResult{}, err
	}

	// Return the decrypted plaintext.
	return actionResult{
		Value: value,
	}, nil
}
