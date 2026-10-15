// Package age implements age encryption with X25519 identities.
package age

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	filippoage "filippo.io/age"

	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/value"
)

// Algorithm identifies the age encryption algorithm.
const Algorithm = "age"

const (
	publicKeyPrefix  = "age-public-key:"
	privateKeyPrefix = "age-private-key:"
)

// Params reserves a construction-time parameters type for age-specific
// behavior. The zero value is currently the complete configuration.
type Params struct{}

// Provider implements cipher.Provider with age X25519 identities and native age
// ciphertext bytes.
type Provider struct{}

// New constructs a concrete age Provider with the supplied parameters.
func New(Params) (*Provider, error) {
	return &Provider{}, nil
}

// Algorithm identifies the envelope algorithm produced by the age cipher.
func (*Provider) Algorithm() string {
	return Algorithm
}

// Keypair creates an age X25519 identity and returns its textual key forms.
func (*Provider) Keypair() (value.Keypair, error) {
	identity, err := filippoage.GenerateX25519Identity()
	if err != nil {
		return value.Keypair{}, fmt.Errorf("generate age keypair: %w", err)
	}

	return value.Keypair{
		PublicKey:  publicKeyPrefix + identity.Recipient().String(),
		PrivateKey: privateKeyPrefix + identity.String(),
	}, nil
}

// ValidateKeypair checks that both age keys are valid and represent one identity.
func (*Provider) ValidateKeypair(publicKey, privateKey string) error {
	nativePublicKey, found := strings.CutPrefix(publicKey, publicKeyPrefix)
	if !found {
		return fmt.Errorf("%w: not an age public key", cipher.ErrInvalidKey)
	}
	recipient, err := filippoage.ParseX25519Recipient(nativePublicKey)
	if err != nil {
		return fmt.Errorf("%w: parse age public key: %w", cipher.ErrInvalidKey, err)
	}

	nativePrivateKey, found := strings.CutPrefix(privateKey, privateKeyPrefix)
	if !found {
		return fmt.Errorf("%w: not an age private key", cipher.ErrInvalidKey)
	}
	identity, err := filippoage.ParseX25519Identity(nativePrivateKey)
	if err != nil {
		return fmt.Errorf("%w: parse age private key: %w", cipher.ErrInvalidKey, err)
	}

	if recipient.String() != identity.Recipient().String() {
		return fmt.Errorf(
			"%w: age public and private keys do not match", cipher.ErrInvalidKey,
		)
	}
	return nil
}

// Encrypt encrypts plaintext for an age X25519 recipient and returns native
// age ciphertext bytes.
func (*Provider) Encrypt(plaintext, publicKey string) ([]byte, error) {
	nativePublicKey, found := strings.CutPrefix(publicKey, publicKeyPrefix)
	if !found {
		return nil, fmt.Errorf("%w: not an age public key", cipher.ErrInvalidKey)
	}
	recipient, err := filippoage.ParseX25519Recipient(nativePublicKey)
	if err != nil {
		return nil, fmt.Errorf("%w: parse age public key: %w", cipher.ErrInvalidKey, err)
	}

	var ciphertext bytes.Buffer
	encryptedWriter, err := filippoage.Encrypt(&ciphertext, recipient)
	if err != nil {
		return nil, fmt.Errorf("create age ciphertext: %w", err)
	}

	if _, err := io.WriteString(encryptedWriter, plaintext); err != nil {
		_ = encryptedWriter.Close()
		return nil, fmt.Errorf("write age ciphertext: %w", err)
	}
	if err := encryptedWriter.Close(); err != nil {
		return nil, fmt.Errorf("close age ciphertext: %w", err)
	}

	return append([]byte(nil), ciphertext.Bytes()...), nil
}

// Decrypt decrypts native age ciphertext bytes with an age X25519 identity.
func (*Provider) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	nativePrivateKey, found := strings.CutPrefix(privateKey, privateKeyPrefix)
	if !found {
		return "", fmt.Errorf("%w: not an age private key", cipher.ErrInvalidKey)
	}
	identity, err := filippoage.ParseX25519Identity(nativePrivateKey)
	if err != nil {
		return "", fmt.Errorf("%w: parse age private key: %w", cipher.ErrInvalidKey, err)
	}

	decryptedReader, err := filippoage.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return "", fmt.Errorf("%w: %w", cipher.ErrDecrypt, err)
	}

	plaintext, err := io.ReadAll(decryptedReader)
	if err != nil {
		return "", fmt.Errorf("%w: read age plaintext: %w", cipher.ErrDecrypt, err)
	}
	return string(plaintext), nil
}
