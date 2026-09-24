package generate

import (
	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/secrets"
)

// actionParams are the inputs to the keypair generation workflow.
type actionParams struct {
	// Group identifies the secret group receiving the generated keypair.
	Group string
}

// actionResult contains safe metadata from a generated keypair.
type actionResult struct {
	// Metadata contains the group, public key, and safe private-key status.
	Metadata secrets.KeypairMetadata
	// SecretsPath is the store path receiving the public key.
	SecretsPath string
	// KeysPath is the private-key path receiving the private key.
	KeysPath string
}

// execute runs the manager's safe missing-identity workflow.
func execute(p actionParams, in *core.Input) (actionResult, error) {
	resolved, err := core.ResolveWorkspace(in)
	if err != nil {
		return actionResult{}, err
	}

	secretManager, err := core.NewSecretsManager(
		resolved.Secrets,
		resolved.Cipher,
	)
	if err != nil {
		return actionResult{}, err
	}
	metadata, err := secretManager.GenerateKeypair(p.Group)
	if err != nil {
		return actionResult{}, err
	}
	return actionResult{
		Metadata:    metadata,
		SecretsPath: resolved.Secrets.SecretsPath,
		KeysPath:    resolved.Secrets.KeysPath,
	}, nil
}
