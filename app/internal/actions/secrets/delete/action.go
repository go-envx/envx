package delete

import (
	"strings"

	"github.com/go-envx/envx/app/internal/config"
)

// actionParams identifies the one secret being removed.
type actionParams struct {
	// Group identifies the case-insensitive key group.
	Group string
	// Key identifies the case-sensitive secret entry.
	Key string
}

// actionResult carries only safe mutation metadata to the renderer.
type actionResult struct {
	// Group is the normalized key group.
	Group string
	// Key is the removed secret entry name.
	Key string
	// StorePath is the workspace store that was updated.
	StorePath string
}

// execute removes one stored secret and delegates the mutation to the root
// secrets manager. The group's keypair and remaining values are preserved.
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

	// Remove the requested secret from the store.
	if err := manager.Delete(p.Group, p.Key); err != nil {
		return actionResult{}, err
	}

	// Return the removed secret's identity and store location.
	return actionResult{
		Group:     strings.ToLower(p.Group),
		Key:       p.Key,
		StorePath: c.Secrets.SecretsPath,
	}, nil
}
