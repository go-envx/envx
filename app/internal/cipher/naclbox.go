package cipher

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/nacl/box"
)

const (
	naclBoxPublicKeyPrefix  = "nacl-box-public-key:"
	naclBoxPrivateKeyPrefix = "nacl-box-private-key:"
	naclBoxKeySize          = 32
)

// NaClBoxOptions reserves a construction-time options type for NaCl Box
// behavior. The zero value is currently the complete configuration.
type NaClBoxOptions struct{}

// algorithmOptions marks NaClBoxOptions as valid input to New.
func (NaClBoxOptions) algorithmOptions() {}

// newNaClBoxCipher constructs a NaCl Box cipher with the supplied options.
func newNaClBoxCipher(NaClBoxOptions) (Cipher, error) {
	return naclBoxCipher{}, nil
}

// naclBoxCipher implements Cipher with anonymous NaCl sealed boxes.
type naclBoxCipher struct{}

// Algorithm identifies the envelope algorithm produced by the NaCl Box cipher.
func (naclBoxCipher) Algorithm() Algorithm {
	return NaClBox
}

// Keypair creates a NaCl Box keypair and returns self-identifying textual keys.
func (naclBoxCipher) Keypair() (Keypair, error) {
	publicKey, privateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return Keypair{}, fmt.Errorf("generate NaCl Box keypair: %w", err)
	}

	return Keypair{
		PublicKey:  encodeNaClBoxPublicKey(publicKey),
		PrivateKey: encodeNaClBoxPrivateKey(publicKey, privateKey),
	}, nil
}

// ValidateKeypair checks that both NaCl Box keys are valid and correspond.
func (naclBoxCipher) ValidateKeypair(publicKey, privateKey string) error {
	nativePublicKey, err := decodeNaClBoxPublicKey(publicKey)
	if err != nil {
		return err
	}
	privatePublicKey, _, err := decodeNaClBoxPrivateKey(privateKey)
	if err != nil {
		return err
	}

	if *nativePublicKey != *privatePublicKey {
		return fmt.Errorf("NaCl Box public and private keys do not match")
	}
	return nil
}

// Encrypt encrypts plaintext for a NaCl Box recipient and returns native sealed
// box bytes.
func (naclBoxCipher) Encrypt(plaintext, publicKey string) ([]byte, error) {
	recipient, err := decodeNaClBoxPublicKey(publicKey)
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
func (naclBoxCipher) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	publicKey, privateKeyBytes, err := decodeNaClBoxPrivateKey(privateKey)
	if err != nil {
		return "", err
	}

	plaintext, ok := box.OpenAnonymous(nil, ciphertext, publicKey, privateKeyBytes)
	if !ok {
		return "", fmt.Errorf("decrypt with NaCl Box: authentication failed")
	}
	return string(plaintext), nil
}

// encodeNaClBoxPublicKey encodes a public key with its key-type marker.
func encodeNaClBoxPublicKey(publicKey *[naclBoxKeySize]byte) string {
	return naclBoxPublicKeyPrefix + base64.RawURLEncoding.EncodeToString(publicKey[:])
}

// encodeNaClBoxPrivateKey encodes the private key with its public counterpart
// because anonymous-box decryption requires both values.
func encodeNaClBoxPrivateKey(
	publicKey, privateKey *[naclBoxKeySize]byte,
) string {
	keyMaterial := make([]byte, 0, naclBoxKeySize*2)
	keyMaterial = append(keyMaterial, publicKey[:]...)
	keyMaterial = append(keyMaterial, privateKey[:]...)
	return naclBoxPrivateKeyPrefix + base64.RawURLEncoding.EncodeToString(keyMaterial)
}

// decodeNaClBoxPublicKey decodes and validates a marked public key.
func decodeNaClBoxPublicKey(value string) (*[naclBoxKeySize]byte, error) {
	keyMaterial, err := decodeNaClBoxKey(value, naclBoxPublicKeyPrefix, "public")
	if err != nil {
		return nil, err
	}

	var publicKey [naclBoxKeySize]byte
	copy(publicKey[:], keyMaterial)
	return &publicKey, nil
}

// decodeNaClBoxPrivateKey decodes a private key bundle into its public and
// private components.
func decodeNaClBoxPrivateKey(
	value string,
) (publicKey, privateKey *[naclBoxKeySize]byte, err error) {
	keyMaterial, err := decodeNaClBoxKey(value, naclBoxPrivateKeyPrefix, "private")
	if err != nil {
		return nil, nil, err
	}

	publicKey = &[naclBoxKeySize]byte{}
	privateKey = &[naclBoxKeySize]byte{}
	copy(publicKey[:], keyMaterial[:naclBoxKeySize])
	copy(privateKey[:], keyMaterial[naclBoxKeySize:])
	return publicKey, privateKey, nil
}

// decodeNaClBoxKey decodes a marked key and validates its exact byte length.
func decodeNaClBoxKey(value, prefix, kind string) ([]byte, error) {
	encoded, found := strings.CutPrefix(value, prefix)
	if !found {
		return nil, fmt.Errorf("invalid NaCl Box %s key", kind)
	}

	keyMaterial, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(keyMaterial) != lenForNaClBoxKey(kind) {
		return nil, fmt.Errorf("invalid NaCl Box %s key", kind)
	}
	return keyMaterial, nil
}

// lenForNaClBoxKey returns the encoded byte length required for a key kind.
func lenForNaClBoxKey(kind string) int {
	if kind == "private" {
		return naclBoxKeySize * 2
	}
	return naclBoxKeySize
}
