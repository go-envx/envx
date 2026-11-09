package secrets

import "github.com/go-envx/envx/app/internal/lib/crypt"

// Reader reads one group snapshot.
type Reader interface {
	Read(group string) (Snapshot, error)
}

// Store persists groups with revision-checked commits.
type Store interface {
	Reader
	Location() string
	List() ([]Snapshot, error)
	// Commit atomically replaces one group when expected is still current and
	// returns ErrConflict otherwise.
	Commit(expected Snapshot, replacement Group) error
}

// KeySource resolves a group's private key, preferring the key matching
// publicKey. Unavailable keys wrap ErrKeyNotAvailable; malformed key material
// wraps ErrInvalidKey.
type KeySource interface {
	Resolve(group, publicKey string) (PrivateKey, error)
}

// Keys resolves and durably retains private keys.
type Keys interface {
	KeySource
	Location() string
	Retain(group string, pair crypt.Keypair) error
}

// Decrypter decrypts native ciphertext for one algorithm.
type Decrypter interface {
	Algorithm() string
	Decrypt(ciphertext []byte, privateKey string) (string, error)
}

// Cipher is a complete cryptographic engine.
type Cipher interface {
	Decrypter
	Keypair() (crypt.Keypair, error)
	ValidateKeypair(publicKey, privateKey string) error
	Encrypt(plaintext, publicKey string) ([]byte, error)
}
