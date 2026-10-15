// Package naclbox implements NaCl sealed-box encryption using Curve25519.
package naclbox

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/nacl/box"

	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/value"
)

// Algorithm identifies the NaCl Box encryption algorithm.
const Algorithm = "nacl-box"

const (
	publicKeyPrefix  = "nacl-box-public-key:"
	privateKeyPrefix = "nacl-box-private-key:"
	keySize          = 32
)

// Params reserves a construction-time parameters type for NaCl Box
// behavior. The zero value is currently the complete configuration.
type Params struct{}

// Provider implements cipher.Provider with anonymous NaCl sealed boxes.
type Provider struct{}

// New constructs a concrete NaCl Box Provider with the supplied parameters.
func New(Params) (*Provider, error) {
	return &Provider{}, nil
}

// Algorithm identifies the envelope algorithm produced by the NaCl Box cipher.
func (*Provider) Algorithm() string {
	return Algorithm
}

// Keypair creates a NaCl Box keypair and returns self-identifying textual keys.
func (*Provider) Keypair() (value.Keypair, error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return value.Keypair{}, fmt.Errorf("generate NaCl Box keypair: %w", err)
	}

	return value.Keypair{
		PublicKey:  encodePublicKey(pub),
		PrivateKey: encodePrivateKey(pub, priv),
	}, nil
}

// ValidateKeypair checks that both NaCl Box keys are valid and correspond.
func (*Provider) ValidateKeypair(publicKey, privateKey string) error {
	nativePublicKey, err := decodePublicKey(publicKey)
	if err != nil {
		return err
	}
	privatePublicKey, _, err := decodePrivateKey(privateKey)
	if err != nil {
		return err
	}

	if *nativePublicKey != *privatePublicKey {
		return fmt.Errorf(
			"%w: NaCl Box public and private keys do not match", cipher.ErrInvalidKey,
		)
	}
	return nil
}

// Encrypt encrypts plaintext for a NaCl Box recipient and returns native sealed
// box bytes.
func (*Provider) Encrypt(plaintext, publicKey string) ([]byte, error) {
	recipient, err := decodePublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	ciphertext, err := box.SealAnonymous(nil, []byte(plaintext), recipient, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("encrypt with NaCl Box: %w", err)
	}
	return ciphertext, nil
}

// Decrypt decrypts native NaCl sealed-box bytes with a private key bundle.
func (*Provider) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	publicKey, privateKeyBytes, err := decodePrivateKey(privateKey)
	if err != nil {
		return "", err
	}

	plaintext, ok := box.OpenAnonymous(nil, ciphertext, publicKey, privateKeyBytes)
	if !ok {
		return "", fmt.Errorf("%w: NaCl Box authentication failed", cipher.ErrDecrypt)
	}
	return string(plaintext), nil
}

// encodePublicKey encodes a public key with its key-type marker.
func encodePublicKey(publicKey *[keySize]byte) string {
	return publicKeyPrefix + base64.RawURLEncoding.EncodeToString(publicKey[:])
}

// encodePrivateKey encodes the private key with its public counterpart
// because anonymous-box decryption requires both values.
func encodePrivateKey(
	publicKey, privateKey *[keySize]byte,
) string {
	keyMaterial := make([]byte, 0, keySize*2)
	keyMaterial = append(keyMaterial, publicKey[:]...)
	keyMaterial = append(keyMaterial, privateKey[:]...)
	return privateKeyPrefix + base64.RawURLEncoding.EncodeToString(keyMaterial)
}

// decodePublicKey decodes and validates a marked public key.
func decodePublicKey(key string) (*[keySize]byte, error) {
	keyMaterial, err := decodeKey(key, publicKeyPrefix, "public")
	if err != nil {
		return nil, err
	}

	var publicKey [keySize]byte
	copy(publicKey[:], keyMaterial)
	return &publicKey, nil
}

// decodePrivateKey decodes a private key bundle into its public and
// private components.
func decodePrivateKey(
	key string,
) (publicKey, privateKey *[keySize]byte, err error) {
	keyMaterial, err := decodeKey(key, privateKeyPrefix, "private")
	if err != nil {
		return nil, nil, err
	}

	publicKey = &[keySize]byte{}
	privateKey = &[keySize]byte{}
	copy(publicKey[:], keyMaterial[:keySize])
	copy(privateKey[:], keyMaterial[keySize:])
	return publicKey, privateKey, nil
}

// decodeKey decodes a marked key and validates its exact byte length.
func decodeKey(key, prefix, kind string) ([]byte, error) {
	encoded, found := strings.CutPrefix(key, prefix)
	if !found {
		return nil, fmt.Errorf("%w: not a NaCl Box %s key", cipher.ErrInvalidKey, kind)
	}

	keyMaterial, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(keyMaterial) != lenForKey(kind) {
		return nil, fmt.Errorf("%w: malformed NaCl Box %s key", cipher.ErrInvalidKey, kind)
	}
	return keyMaterial, nil
}

// lenForKey returns the encoded byte length required for a key kind.
func lenForKey(kind string) int {
	if kind == "private" {
		return keySize * 2
	}
	return keySize
}
