package secrets

import (
	"errors"
	"strings"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/privatekey"
)

// Params supplies paths and dependencies for a Manager.
type Params struct {
	// SecretsPath is the absolute path of the secrets store.
	SecretsPath string
	// KeysPath is the absolute path of the private-key file.
	KeysPath string
	// Cipher performs key generation and keypair validation.
	Cipher cipher.Cipher
	// PrivateKeyResolver resolves private-key material for read operations.
	PrivateKeyResolver privatekey.Resolver
	// PrivateKeyDestination receives newly generated private-key material.
	PrivateKeyDestination privatekey.Destination
}

// Manager coordinates secret workflows over the store and key-material ports.
type Manager struct {
	// params holds the validated construction input privately.
	params Params
}

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

	// Assemble the manager with the validated paths and dependencies.
	return &Manager{params: params}, nil
}
