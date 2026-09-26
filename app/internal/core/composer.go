package core

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	pkfilestore "github.com/go-envx/envx/app/internal/features/privatekey/filestore"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/workspace"
	wsfilestore "github.com/go-envx/envx/app/internal/features/workspace/filestore"
	"github.com/go-envx/envx/app/internal/resources/cipher"
)

// NewConfiguredCipher resolves the workspace cipher when a manifest is present,
// or constructs the application's default cipher without a workspace.
func NewConfiguredCipher(in *Input) (cipher.Cipher, error) {
	// Bind the resolved manifest path and conventional filename into a repository.
	wsRepo, err := wsfilestore.New(wsfilestore.Params{
		Path:     resolveManifestPath(in),
		Filename: defaultManifestFilename,
	})
	if err != nil {
		return nil, err
	}

	wsService, err := workspace.NewService(workspace.ServiceParams{
		Repository: wsRepo,
	})
	if err != nil {
		return nil, err
	}

	// Replace the default with the manifest's algorithm when a workspace exists.
	cipherParams := cipher.Params{Algorithm: defaultCipherAlgorithm}
	ws, err := wsService.Load()
	if err != nil {
		if !errors.Is(err, workspace.ErrNotFound) {
			return nil, err
		}
	} else {
		cipherParams = resolveCipherParams(manifestContext{
			workspace: ws,
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

	pkRepo, err := pkfilestore.New(pkfilestore.Params{Path: s.KeysPath})
	if err != nil {
		return nil, fmt.Errorf("creating privatekey repository: %w", err)
	}

	pkService, err := privatekey.NewService(privatekey.ServiceParams{
		Repository: pkRepo,
		LookupEnv:  os.LookupEnv,
	})
	if err != nil {
		return nil, fmt.Errorf("creating privatekey service: %w", err)
	}

	// Add the workspace's cipher and private-key service.
	s.Cipher = oCipher
	s.PrivateKeyService = pkService

	// Return the fully wired secrets manager.
	return secrets.New(s)
}
