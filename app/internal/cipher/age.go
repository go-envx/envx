package cipher

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
)

const (
	agePublicKeyPrefix  = "age-public-key:"
	agePrivateKeyPrefix = "age-private-key:"
)

// -------------------------------------------------------------------------------------

// AgeOptions reserves a construction-time options type for age-specific
// behavior. The zero value is currently the complete configuration.
type AgeOptions struct{}

// -------------------------------------------------------------------------------------

// algorithmOptions marks AgeOptions as valid input to New.
func (AgeOptions) algorithmOptions() {}

// -------------------------------------------------------------------------------------

// newAgeCipher constructs an age cipher with the supplied options.
func newAgeCipher(AgeOptions) (Cipher, error) {
	return ageCipher{}, nil
}

// -------------------------------------------------------------------------------------

// ageCipher implements Cipher with age X25519 identities and native age
// ciphertext bytes.
type ageCipher struct{}

// -------------------------------------------------------------------------------------

// Keypair creates an age X25519 identity and returns its textual key forms.
func (ageCipher) Keypair() (Keypair, error) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return Keypair{}, fmt.Errorf("generate age keypair: %w", err)
	}

	return Keypair{
		PublicKey:  agePublicKeyPrefix + identity.Recipient().String(),
		PrivateKey: agePrivateKeyPrefix + identity.String(),
	}, nil
}

// -------------------------------------------------------------------------------------

// ValidateKeypair checks that both age keys are valid and represent one identity.
func (ageCipher) ValidateKeypair(publicKey, privateKey string) error {
	nativePublicKey, found := strings.CutPrefix(publicKey, agePublicKeyPrefix)
	if !found {
		return fmt.Errorf("invalid age public key")
	}
	recipient, err := age.ParseX25519Recipient(nativePublicKey)
	if err != nil {
		return fmt.Errorf("parse age public key: %w", err)
	}

	nativePrivateKey, found := strings.CutPrefix(privateKey, agePrivateKeyPrefix)
	if !found {
		return fmt.Errorf("invalid age private key")
	}
	identity, err := age.ParseX25519Identity(nativePrivateKey)
	if err != nil {
		return fmt.Errorf("parse age private key: %w", err)
	}

	if recipient.String() != identity.Recipient().String() {
		return fmt.Errorf("age public and private keys do not match")
	}
	return nil
}

// -------------------------------------------------------------------------------------

// Encrypt encrypts plaintext for an age X25519 recipient and returns native
// age ciphertext bytes.
func (ageCipher) Encrypt(plaintext, publicKey string) ([]byte, error) {
	nativePublicKey, found := strings.CutPrefix(publicKey, agePublicKeyPrefix)
	if !found {
		return nil, fmt.Errorf("invalid age public key")
	}
	recipient, err := age.ParseX25519Recipient(nativePublicKey)
	if err != nil {
		return nil, fmt.Errorf("parse age public key: %w", err)
	}

	var ciphertext bytes.Buffer
	encryptedWriter, err := age.Encrypt(&ciphertext, recipient)
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

// -------------------------------------------------------------------------------------

// Decrypt decrypts native age ciphertext bytes with an age X25519 identity.
func (ageCipher) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	nativePrivateKey, found := strings.CutPrefix(privateKey, agePrivateKeyPrefix)
	if !found {
		return "", fmt.Errorf("invalid age private key")
	}
	identity, err := age.ParseX25519Identity(nativePrivateKey)
	if err != nil {
		return "", fmt.Errorf("parse age private key: %w", err)
	}

	decryptedReader, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return "", fmt.Errorf("decrypt age ciphertext: %w", err)
	}

	plaintext, err := io.ReadAll(decryptedReader)
	if err != nil {
		return "", fmt.Errorf("read age plaintext: %w", err)
	}
	return string(plaintext), nil
}
