package decrypt

import (
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/secrets"
)

// actionParams selects which stored secrets to decrypt. An empty group or key
// widens the selection to every value in that dimension.
type actionParams struct {
	// Group limits the operation to one key group; empty matches all groups.
	Group string
	// Key limits the operation to one secret key; empty matches all keys.
	Key string
}

// actionResult carries only safe mutation metadata to the renderer.
type actionResult struct {
	// Changed lists the identities whose values were decrypted.
	Changed []secrets.SecretReference
	// Unavailable lists groups skipped because no private key was available.
	Unavailable []string
	// StorePath is the workspace store that was updated.
	StorePath string
}

// execute decrypts the selected ciphertext store entries in place and delegates
// the mutation to the root secrets manager.
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

	// Decrypt every matching ciphertext value in place.
	result, err := manager.Decrypt(p.Group, p.Key)
	if err != nil {
		return actionResult{}, err
	}

	// Carry only the changed identities and the store location to the renderer.
	return actionResult{
		Changed:     result.Secrets,
		Unavailable: result.Unavailable,
		StorePath:   c.Secrets.SecretsPath,
	}, nil
}
