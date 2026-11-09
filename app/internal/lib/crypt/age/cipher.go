// Package age implements age encryption with X25519 identities.
package age

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	agepkg "filippo.io/age"

	"github.com/go-envx/envx/app/internal/lib/crypt"
)

// Algorithm identifies the age encryption algorithm.
const Algorithm = "age"

const (
	publicKeyPrefix  = "age-public-key:"
	privateKeyPrefix = "age-private-key:"
)

// Cipher implements cryptographic operations with age X25519 identities
// and native age ciphertext bytes. The zero value is usable.
type Cipher struct{}

// Algorithm identifies the envelope algorithm produced by the age cipher.
func (Cipher) Algorithm() string {
	return Algorithm
}

// Keypair creates an age X25519 identity and returns its textual key forms.
func (Cipher) Keypair() (crypt.Keypair, error) {
	identity, err := agepkg.GenerateX25519Identity()
	if err != nil {
		return crypt.Keypair{}, fmt.Errorf("generate age keypair: %w", err)
	}

	return crypt.Keypair{
		PublicKey:  publicKeyPrefix + identity.Recipient().String(),
		PrivateKey: privateKeyPrefix + identity.String(),
	}, nil
}

// ValidateKeypair checks that both age keys are valid and represent one identity.
func (Cipher) ValidateKeypair(publicKey, privateKey string) error {
	nativePublicKey, found := strings.CutPrefix(publicKey, publicKeyPrefix)
	if !found {
		return fmt.Errorf("%w: not an age public key", crypt.ErrInvalidKey)
	}
	recipient, err := agepkg.ParseX25519Recipient(nativePublicKey)
	if err != nil {
		return fmt.Errorf("%w: parse age public key: %w", crypt.ErrInvalidKey, err)
	}

	nativePrivateKey, found := strings.CutPrefix(privateKey, privateKeyPrefix)
	if !found {
		return fmt.Errorf("%w: not an age private key", crypt.ErrInvalidKey)
	}
	identity, err := agepkg.ParseX25519Identity(nativePrivateKey)
	if err != nil {
		return fmt.Errorf("%w: parse age private key: %w", crypt.ErrInvalidKey, err)
	}

	if recipient.String() != identity.Recipient().String() {
		return fmt.Errorf(
			"%w: age public and private keys do not match", crypt.ErrInvalidKey,
		)
	}
	return nil
}

// Encrypt encrypts plaintext for an age X25519 recipient and returns native
// age ciphertext bytes.
func (Cipher) Encrypt(plaintext, publicKey string) ([]byte, error) {
	nativePublicKey, found := strings.CutPrefix(publicKey, publicKeyPrefix)
	if !found {
		return nil, fmt.Errorf("%w: not an age public key", crypt.ErrInvalidKey)
	}
	recipient, err := agepkg.ParseX25519Recipient(nativePublicKey)
	if err != nil {
		return nil, fmt.Errorf("%w: parse age public key: %w", crypt.ErrInvalidKey, err)
	}

	var ciphertext bytes.Buffer
	encryptedWriter, err := agepkg.Encrypt(&ciphertext, recipient)
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
func (Cipher) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	nativePrivateKey, found := strings.CutPrefix(privateKey, privateKeyPrefix)
	if !found {
		return "", fmt.Errorf("%w: not an age private key", crypt.ErrInvalidKey)
	}
	identity, err := agepkg.ParseX25519Identity(nativePrivateKey)
	if err != nil {
		return "", fmt.Errorf("%w: parse age private key: %w", crypt.ErrInvalidKey, err)
	}

	decryptedReader, err := agepkg.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return "", fmt.Errorf("%w: %w", crypt.ErrDecrypt, err)
	}

	plaintext, err := io.ReadAll(decryptedReader)
	if err != nil {
		return "", fmt.Errorf("%w: read age plaintext: %w", crypt.ErrDecrypt, err)
	}
	return string(plaintext), nil
}
