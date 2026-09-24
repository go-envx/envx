package inspect

import (
	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/secrets"
)

// actionParams are the inputs to the keypair inspection workflow.
type actionParams struct {
	// Group identifies the secret group to inspect.
	Group string
}

// execute runs the manager's non-mutating keypair inspection workflow.
func execute(p actionParams, in *core.Input) (secrets.KeypairMetadata, error) {
	resolved, err := core.ResolveWorkspace(in)
	if err != nil {
		return secrets.KeypairMetadata{}, err
	}
	secretManager, err := core.NewSecretsManager(
		resolved.Secrets,
		resolved.Cipher,
	)
	if err != nil {
		return secrets.KeypairMetadata{}, err
	}
	metadata, err := secretManager.InspectKeypair(p.Group)
	if err != nil {
		return secrets.KeypairMetadata{}, err
	}
	return metadata, nil
}
