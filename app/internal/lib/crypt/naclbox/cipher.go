// Package naclbox implements NaCl sealed-box encryption using Curve25519.
package naclbox

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	boxpkg "golang.org/x/crypto/nacl/box"

	"github.com/go-envx/envx/app/internal/lib/crypt"
)

// Algorithm identifies the NaCl Box encryption algorithm.
const Algorithm = "nacl-box"

const (
	publicKeyPrefix  = "nacl-box-public-key:"
	privateKeyPrefix = "nacl-box-private-key:"
	keySize          = 32
)

// Cipher implements cryptographic operations with anonymous NaCl sealed boxes.
// The zero value is usable.
type Cipher struct{}

// Algorithm identifies the envelope algorithm produced by the NaCl Box cipher.
func (Cipher) Algorithm() string {
	return Algorithm
}

// Keypair creates a NaCl Box keypair and returns self-identifying textual keys.
func (Cipher) Keypair() (crypt.Keypair, error) {
	pub, priv, err := boxpkg.GenerateKey(rand.Reader)
	if err != nil {
		return crypt.Keypair{}, fmt.Errorf("generate NaCl Box keypair: %w", err)
	}

	privKeyMaterial := make([]byte, 0, keySize*2)
	privKeyMaterial = append(privKeyMaterial, pub[:]...)
	privKeyMaterial = append(privKeyMaterial, priv[:]...)

	return crypt.Keypair{
		PublicKey:  publicKeyPrefix + base64.RawURLEncoding.EncodeToString(pub[:]),
		PrivateKey: privateKeyPrefix + base64.RawURLEncoding.EncodeToString(privKeyMaterial),
	}, nil
}

// ValidateKeypair checks that both NaCl Box keys are valid and correspond.
func (Cipher) ValidateKeypair(publicKey, privateKey string) error {
	pub, err := decodeKey(publicKey, publicKeyPrefix, "public", keySize)
	if err != nil {
		return err
	}
	privBundle, err := decodeKey(
		privateKey, privateKeyPrefix, "private", keySize*2,
	)
	if err != nil {
		return err
	}

	if !bytes.Equal(pub, privBundle[:keySize]) {
		return fmt.Errorf(
			"%w: NaCl Box public and private keys do not match",
			crypt.ErrInvalidKey,
		)
	}
	return nil
}

// Encrypt encrypts plaintext for a NaCl Box recipient and returns native sealed
// box bytes.
func (Cipher) Encrypt(plaintext, publicKey string) ([]byte, error) {
	keyBytes, err := decodeKey(publicKey, publicKeyPrefix, "public", keySize)
	if err != nil {
		return nil, err
	}

	var recipient [keySize]byte
	copy(recipient[:], keyBytes)

	ciphertext, err := boxpkg.SealAnonymous(
		nil, []byte(plaintext), &recipient, rand.Reader,
	)
	if err != nil {
		return nil, fmt.Errorf("encrypt with NaCl Box: %w", err)
	}
	return ciphertext, nil
}

// Decrypt decrypts native NaCl sealed-box bytes with a private key bundle.
func (Cipher) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	bundle, err := decodeKey(
		privateKey, privateKeyPrefix, "private", keySize*2,
	)
	if err != nil {
		return "", err
	}

	var pub, priv [keySize]byte
	copy(pub[:], bundle[:keySize])
	copy(priv[:], bundle[keySize:])

	plaintext, ok := boxpkg.OpenAnonymous(nil, ciphertext, &pub, &priv)
	if !ok {
		return "", fmt.Errorf(
			"%w: NaCl Box authentication failed", crypt.ErrDecrypt,
		)
	}
	return string(plaintext), nil
}

// decodeKey decodes a marked key and validates its exact byte length.
func decodeKey(key, prefix, kind string, expectedLen int) ([]byte, error) {
	encoded, found := strings.CutPrefix(key, prefix)
	if !found {
		return nil, fmt.Errorf(
			"%w: not a NaCl Box %s key", crypt.ErrInvalidKey, kind,
		)
	}

	keyMaterial, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(keyMaterial) != expectedLen {
		return nil, fmt.Errorf(
			"%w: malformed NaCl Box %s key", crypt.ErrInvalidKey, kind,
		)
	}
	return keyMaterial, nil
}
