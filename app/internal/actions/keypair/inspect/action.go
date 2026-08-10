package inspect

import (
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/secrets"
)

// actionParams are the inputs to the keypair inspection workflow.
type actionParams struct {
	// Group identifies the secret group to inspect.
	Group string
}

// execute runs the manager's non-mutating keypair inspection workflow.
func execute(p actionParams, in *config.Input) (secrets.KeypairMetadata, error) {
	resolved, err := config.ResolveWorkspace(in)
	if err != nil {
		return secrets.KeypairMetadata{}, err
	}
	secretManager, err := config.NewSecretsManager(
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
