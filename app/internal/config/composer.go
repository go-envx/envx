package config

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/manifest"
	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets"
)

// NewConfiguredCipher resolves the workspace cipher when a manifest is present,
// or constructs the application's default cipher without a workspace.
func NewConfiguredCipher(in *Input) (cipher.Cipher, error) {
	// Load an optional manifest so standalone commands can use the default cipher.
	m, _, err := manifest.LoadOptional(resolveManifestPath(in))
	if err != nil {
		return nil, err
	}

	// Replace the default with the manifest's algorithm when a workspace exists.
	params := cipher.Params{Algorithm: defaultCipherAlgorithm}
	if m != nil {
		params = resolveCipherParams(manifestContext{manifest: m})
	}

	// Construct the selected implementation and add context if construction fails.
	selectedCipher, err := cipher.New(params)
	if err != nil {
		return nil, fmt.Errorf("creating configured cipher: %w", err)
	}
	return selectedCipher, nil
}

// NewSecretsManager composes the configured cipher and private-key ports into
// a secrets manager for one resolved workspace.
func NewSecretsManager(
	params secrets.Params,
	cipherParams cipher.Params,
) (*secrets.Manager, error) {
	// Construct the configured cipher before wiring it into the secrets manager.
	selectedCipher, err := cipher.New(cipherParams)
	if err != nil {
		return nil, fmt.Errorf("creating configured cipher: %w", err)
	}

	// Add the workspace's private-key resolver and file destination ports.
	params.Cipher = selectedCipher
	params.PrivateKeyResolver = privatekey.NewResolver(
		privatekey.ResolverOptions{KeysPath: params.KeysPath},
	)
	params.PrivateKeyDestination = privatekey.NewFileDestination(params.KeysPath)

	// Return the fully wired secrets manager.
	return secrets.New(params)
}
