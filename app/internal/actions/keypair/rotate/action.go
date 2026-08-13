package rotate

import (
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/secrets"
)

// actionParams are the inputs to the keypair rotation workflow.
type actionParams struct {
	// Group identifies the secret group whose identity is replaced.
	Group string
}

// actionResult contains safe metadata from a completed rotation.
type actionResult struct {
	// Result reports the new keypair and re-encrypted secret identities.
	Result secrets.UpdateResult
	// SecretsPath is the store path holding the new public key and values.
	SecretsPath string
	// KeysPath is the private-key path receiving the new private key.
	KeysPath string
}

// execute runs the manager's identity-replacement and re-encryption workflow.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	resolved, err := config.ResolveWorkspace(in)
	if err != nil {
		return actionResult{}, err
	}

	secretManager, err := config.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		return actionResult{}, err
	}
	result, err := secretManager.RotateKeypair(p.Group)
	if err != nil {
		return actionResult{}, err
	}
	return actionResult{
		Result:      result,
		SecretsPath: resolved.Secrets.SecretsPath,
		KeysPath:    resolved.Secrets.KeysPath,
	}, nil
}
