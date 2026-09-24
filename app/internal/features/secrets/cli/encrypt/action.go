package encrypt

import (
	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/secrets"
)

// actionParams selects which stored secrets to encrypt. An empty group or key
// widens the selection to every value in that dimension.
type actionParams struct {
	// Group limits the operation to one key group; empty matches all groups.
	Group string
	// Key limits the operation to one secret key; empty matches all keys.
	Key string
}

// actionResult carries only safe mutation metadata to the renderer.
type actionResult struct {
	// Changed lists the identities whose values were encrypted.
	Changed []secrets.SecretReference
	// StorePath is the workspace store that was updated.
	StorePath string
}

// execute encrypts the selected plaintext store entries in place and delegates
// the mutation to the root secrets manager.
func execute(p actionParams, in *core.Input) (actionResult, error) {
	// Resolve the workspace configuration and encryption settings.
	c, err := core.ResolveWorkspace(in)
	if err != nil {
		return actionResult{}, err
	}
	// Create the manager responsible for encrypted secret storage.
	manager, err := core.NewSecretsManager(c.Secrets, c.Cipher)
	if err != nil {
		return actionResult{}, err
	}

	// Encrypt every matching plaintext value in place.
	result, err := manager.Encrypt(p.Group, p.Key)
	if err != nil {
		return actionResult{}, err
	}

	// Carry only the changed identities and the store location to the renderer.
	return actionResult{
		Changed:   result.Secrets,
		StorePath: c.Secrets.SecretsPath,
	}, nil
}
