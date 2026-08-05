package secrets

import (
	"errors"
	"strings"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/privatekey"
)

// -------------------------------------------------------------------------------------

// Manager coordinates secret workflows over the store and key-material ports.
type Manager struct {
	// secretsPath is the bound secrets.yaml path.
	secretsPath string
	// keysPath is the configured local private-key path.
	keysPath string
	// cipher performs key generation and validation.
	cipher cipher.Cipher
	// privateKeyResolver resolves private-key material.
	privateKeyResolver privatekey.Resolver
	// privateKeyDestination receives generated private-key material.
	privateKeyDestination privatekey.Destination
}

// -------------------------------------------------------------------------------------

// New binds paths and dependencies into a manager. All paths and operational
// dependencies must be supplied by the composition layer.
func New(params Params) (*Manager, error) {
	// Require a path for the secrets store.
	if strings.TrimSpace(params.SecretsPath) == "" {
		return nil, errors.New("secrets path is empty")
	}

	// Require the private-key path from the configuration layer.
	if strings.TrimSpace(params.KeysPath) == "" {
		return nil, errors.New("keys path is empty")
	}

	// Require the cipher from the composition layer.
	if params.Cipher == nil {
		return nil, errors.New("cipher is nil")
	}

	// Require the private-key resolver from the composition layer.
	if params.PrivateKeyResolver == nil {
		return nil, errors.New("private-key resolver is nil")
	}

	// Require the private-key destination from the composition layer.
	if params.PrivateKeyDestination == nil {
		return nil, errors.New("private-key destination is nil")
	}

	// Assemble the manager with resolved paths and dependencies.
	return &Manager{
		secretsPath:           params.SecretsPath,
		keysPath:              params.KeysPath,
		cipher:                params.Cipher,
		privateKeyResolver:    params.PrivateKeyResolver,
		privateKeyDestination: params.PrivateKeyDestination,
	}, nil
}
