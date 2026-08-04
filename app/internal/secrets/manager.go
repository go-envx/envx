package secrets

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/secrets/internal/privatekey"
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

// New binds paths and dependencies into a manager. Missing cipher, resolver, and
// destination dependencies select the safe local age, environment/file, and
// configured-file defaults respectively.
func New(params Params) (*Manager, error) {
	if strings.TrimSpace(params.SecretsPath) == "" {
		return nil, errors.New("secrets path is empty")
	}

	keysPath := params.KeysPath
	if keysPath == "" {
		keysPath = filepath.Join(filepath.Dir(params.SecretsPath), "envx.keys")
	}

	selectedCipher := params.Cipher
	if selectedCipher == nil {
		var err error
		selectedCipher, err = cipher.New(cipher.DefaultAlgorithm, cipher.AgeOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating default cipher: %w", err)
		}
	}

	selectedResolver := params.PrivateKeyResolver
	if selectedResolver == nil {
		selectedResolver = privatekey.NewResolver(privatekey.ResolverOptions{
			KeysPath: keysPath,
		})
	}

	selectedDestination := params.PrivateKeyDestination
	if selectedDestination == nil {
		selectedDestination = privatekey.NewFileDestination(keysPath)
	}

	return &Manager{
		secretsPath:           params.SecretsPath,
		keysPath:              keysPath,
		cipher:                selectedCipher,
		privateKeyResolver:    selectedResolver,
		privateKeyDestination: selectedDestination,
	}, nil
}
