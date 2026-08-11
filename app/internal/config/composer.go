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
	// Bind the resolved manifest path and conventional filename into a manager.
	manifestManager, err := manifest.New(manifest.Params{
		Path:     resolveManifestPath(in),
		Filename: defaultManifestFilename,
	})
	if err != nil {
		return nil, err
	}

	// Detect an optional manifest so standalone commands can use the default cipher.
	manifestExists, err := manifestManager.Exists()
	if err != nil {
		return nil, err
	}

	// Replace the default with the manifest's algorithm when a workspace exists.
	cipherParams := cipher.Params{Algorithm: defaultCipherAlgorithm}
	if manifestExists {
		manifestDocument, err := manifestManager.Load()
		if err != nil {
			return nil, err
		}
		cipherParams = resolveCipherParams(manifestContext{
			manifest: manifestDocument.Content,
		})
	}

	// Construct the selected implementation and add context if construction fails.
	selectedCipher, err := cipher.New(cipherParams)
	if err != nil {
		return nil, fmt.Errorf("creating configured cipher: %w", err)
	}
	return selectedCipher, nil
}

// NewSecretsManager composes the configured cipher and private-key ports into
// a secrets manager for one resolved workspace.
func NewSecretsManager(s secrets.Params, c cipher.Params) (*secrets.Manager, error) {
	// Construct the configured cipher before wiring it into the secrets manager.
	oCipher, err := cipher.New(c)
	if err != nil {
		return nil, fmt.Errorf("creating configured cipher: %w", err)
	}

	// Add the workspace's private-key resolver and file destination ports.
	s.Cipher = oCipher
	s.PrivateKeyResolver = privatekey.NewResolver(
		privatekey.ResolverOptions{KeysPath: s.KeysPath},
	)
	s.PrivateKeyDestination = privatekey.NewFileDestination(s.KeysPath)

	// Return the fully wired secrets manager.
	return secrets.New(s)
}
