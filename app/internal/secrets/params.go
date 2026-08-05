package secrets

import (
	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/privatekey"
)

// -------------------------------------------------------------------------------------

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
