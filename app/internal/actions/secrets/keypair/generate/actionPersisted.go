package generate

import (
	"fmt"
	"io"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/secrets"
)

// -------------------------------------------------------------------------------------

// actionPersistedResult contains safe metadata from a generated keypair.
type actionPersistedResult struct {
	// Metadata contains the group, public key, and safe private-key status.
	Metadata secrets.KeypairMetadata
	// SecretsPath is the store path receiving the public key.
	SecretsPath string
	// KeysPath is the private-key path receiving the private key.
	KeysPath string
}

// -------------------------------------------------------------------------------------

// execute runs the manager's safe missing-identity workflow.
func execute(p actionParams, in *config.Input) (actionPersistedResult, error) {
	resolved, err := config.ResolveWorkspace(in)
	if err != nil {
		return actionPersistedResult{}, err
	}

	secretManager, err := config.NewSecretsManager(
		resolved.Secrets,
		resolved.Cipher,
	)
	if err != nil {
		return actionPersistedResult{}, err
	}
	metadata, err := secretManager.GenerateKeypair(p.Group)
	if err != nil {
		return actionPersistedResult{}, err
	}
	return actionPersistedResult{
		Metadata:    metadata,
		SecretsPath: resolved.Secrets.SecretsPath,
		KeysPath:    resolved.Secrets.KeysPath,
	}, nil
}

// -------------------------------------------------------------------------------------

// render prints safe generation metadata without private-key material.
func render(w io.Writer, result actionPersistedResult) error {
	_, err := fmt.Fprintf(
		w,
		"Generated keypair for group %q:\n"+
			"  public key: %s\n"+
			"  secrets store: %s\n"+
			"  private key file: %s\n",
		result.Metadata.Group,
		result.Metadata.PublicKey,
		result.SecretsPath,
		result.KeysPath,
	)
	return err
}
