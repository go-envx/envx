package secrets

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/lib/crypt"
)

// Private-key availability reported by KeypairMetadata.
const (
	PrivateKeyNotAvailable = "not_available"
	PrivateKeyValid        = "valid"
	PrivateKeyInvalid      = "invalid"
)

// KeypairMetadata reports public-key state without private-key material.
type KeypairMetadata struct {
	// Group is the lowercase group name.
	Group string
	// PublicKey is the group's stored public key.
	PublicKey string
	// PrivateKeyStatus is PrivateKeyValid, PrivateKeyInvalid, or
	// PrivateKeyNotAvailable.
	PrivateKeyStatus string
}

// KeypairResult reports a generated or rotated keypair.
type KeypairResult struct {
	// Keypair describes the newly committed public key.
	Keypair KeypairMetadata
	// PublicKeyLocation identifies the store holding the public key.
	PublicKeyLocation string
	// PrivateKeyLocation identifies where the private key was retained.
	PrivateKeyLocation string
	// Reencrypted lists values re-encrypted by a rotation.
	Reencrypted []Reference
}

// GenerateKeypair creates a group identity, retaining the private key before
// committing the matching public key.
func (s *Service) GenerateKeypair(group string) (KeypairResult, error) {
	group, err := NormalizeGroup(group)
	if err != nil {
		return KeypairResult{}, err
	}
	snap, err := s.store.Read(group)
	if err != nil {
		return KeypairResult{}, err
	}
	if snap.Group.PublicKey != "" {
		return KeypairResult{}, fmt.Errorf(
			"group %q already has a public key: %w", group, ErrKeypairExists,
		)
	}
	pair, err := s.newKeypair(group)
	if err != nil {
		return KeypairResult{}, err
	}
	if err := s.keys.Retain(group, pair); err != nil {
		return KeypairResult{}, fmt.Errorf(
			"writing private key for group %q: %w", group, err,
		)
	}
	replacement := s.cloneGroup(snap, group)
	replacement.PublicKey = pair.PublicKey
	if err := s.store.Commit(snap, replacement); err != nil {
		return KeypairResult{}, fmt.Errorf(
			"private key for group %q was written, "+
				"but public key could not be committed: %w",
			group, err,
		)
	}
	return s.keypairResult(group, pair.PublicKey, nil), nil
}

// InspectKeypair reports a group's key status without exposing key material.
func (s *Service) InspectKeypair(group string) (KeypairMetadata, error) {
	group, err := NormalizeGroup(group)
	if err != nil {
		return KeypairMetadata{}, err
	}
	snap, err := s.store.Read(group)
	if err != nil {
		return KeypairMetadata{}, err
	}
	if snap.Group.PublicKey == "" {
		return KeypairMetadata{}, fmt.Errorf(
			"group %q has no public key: %w", group, ErrGroupNotFound,
		)
	}
	return s.inspect(group, snap.Group.PublicKey)
}

// inspect resolves and validates a group's private key against publicKey.
func (s *Service) inspect(group, publicKey string) (KeypairMetadata, error) {
	metadata := KeypairMetadata{
		Group:            group,
		PublicKey:        publicKey,
		PrivateKeyStatus: PrivateKeyNotAvailable,
	}
	key, err := s.keys.Resolve(group, publicKey)
	if errors.Is(err, ErrKeyNotAvailable) {
		return metadata, nil
	}
	if errors.Is(err, ErrInvalidKey) {
		metadata.PrivateKeyStatus = PrivateKeyInvalid
		return metadata, nil
	}
	if err != nil {
		return KeypairMetadata{}, fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}
	if key.Value == "" || s.cipher.ValidateKeypair(publicKey, key.Value) != nil {
		metadata.PrivateKeyStatus = PrivateKeyInvalid
		return metadata, nil
	}
	metadata.PrivateKeyStatus = PrivateKeyValid
	return metadata, nil
}

// newKeypair generates and validates a complete keypair.
func (s *Service) newKeypair(group string) (crypt.Keypair, error) {
	pair, err := s.cipher.Keypair()
	if err != nil {
		return crypt.Keypair{}, fmt.Errorf(
			"generating keypair for group %q: %w", group, err,
		)
	}
	if pair.PublicKey == "" || pair.PrivateKey == "" {
		return crypt.Keypair{}, errors.New("cipher generated an incomplete keypair")
	}
	if err := s.cipher.ValidateKeypair(pair.PublicKey, pair.PrivateKey); err != nil {
		return crypt.Keypair{}, fmt.Errorf("generated keypair is invalid: %w", err)
	}
	return pair, nil
}

// keypairResult reports a newly committed public key with a valid private key.
func (s *Service) keypairResult(
	group, publicKey string, reencrypted []Reference,
) KeypairResult {
	return KeypairResult{
		Keypair: KeypairMetadata{
			Group:            group,
			PublicKey:        publicKey,
			PrivateKeyStatus: PrivateKeyValid,
		},
		PublicKeyLocation:  s.store.Location(),
		PrivateKeyLocation: s.keys.Location(),
		Reencrypted:        reencrypted,
	}
}
