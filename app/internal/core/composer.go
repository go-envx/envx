package core

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-envx/envx/app/internal/features/env"
	envfilestore "github.com/go-envx/envx/app/internal/features/env/filestore"
	"github.com/go-envx/envx/app/internal/features/privatekey"
	pkfilestore "github.com/go-envx/envx/app/internal/features/privatekey/filestore"
	"github.com/go-envx/envx/app/internal/features/secrets"
	secfilestore "github.com/go-envx/envx/app/internal/features/secrets/filestore"
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

// cipherAdapter adapts a cipher.Cipher to the secrets.CipherClient interface.
type cipherAdapter struct {
	cipher cipher.Cipher
}

func (a cipherAdapter) Algorithm() string {
	return string(a.cipher.Algorithm())
}

func (a cipherAdapter) Keypair() (publicKey, privateKey string, err error) {
	kp, err := a.cipher.Keypair()
	if err != nil {
		return "", "", err
	}
	return kp.PublicKey, kp.PrivateKey, nil
}

func (a cipherAdapter) ValidateKeypair(publicKey, privateKey string) error {
	return a.cipher.ValidateKeypair(publicKey, privateKey)
}

func (a cipherAdapter) Encrypt(plaintext, publicKey string) ([]byte, error) {
	return a.cipher.Encrypt(plaintext, publicKey)
}

func (a cipherAdapter) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	return a.cipher.Decrypt(ciphertext, privateKey)
}

// SecretsParams supplies paths and configuration for secrets composition.
type SecretsParams struct {
	SecretsPath   string
	KeysPath      string
	DefaultIndent int
}

// NewSecretsService composes the configured cipher, filestores, and domain services.
func NewSecretsService(
	secretsPath, keysPath string,
	cipherParams cipher.Params,
	indent int,
) (*secrets.Service, error) {
	oCipher, err := cipher.New(cipherParams)
	if err != nil {
		return nil, fmt.Errorf("creating configured cipher: %w", err)
	}

	pkRepo, err := pkfilestore.New(pkfilestore.Params{Path: keysPath})
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

	secStore, err := secfilestore.New(secfilestore.Params{
		Path:          secretsPath,
		DefaultIndent: indent,
	})
	if err != nil {
		return nil, fmt.Errorf("creating secrets store: %w", err)
	}

	return secrets.NewService(secrets.ServiceParams{
		Repository:        secStore,
		Cipher:            cipherAdapter{cipher: oCipher},
		PrivateKeyService: pkService,
	})
}

// NewSecretsManager composes the configured cipher and private-key ports into
// a secrets service for one resolved workspace.
func NewSecretsManager(s SecretsParams, c cipher.Params) (*secrets.Service, error) {
	return NewSecretsService(s.SecretsPath, s.KeysPath, c, s.DefaultIndent)
}

// NewEnvService constructs an env.Service with a local filestore repository.
func NewEnvService(params env.ServiceParams) (*env.Service, error) {
	if params.Repository == nil {
		params.Repository = envfilestore.New(envfilestore.Params{})
	}
	return env.NewService(params)
}
