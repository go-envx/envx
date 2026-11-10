package secrets

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// Service manages stored secrets and keypair lifecycle.
type Service struct {
	// store persists groups and their public keys.
	store Store
	// keys resolves and retains private keys.
	keys Keys
	// cipher generates keypairs and seals and opens values.
	cipher Cipher
}

// New constructs a secrets service. All dependencies are required.
func New(store Store, keys Keys, cipher Cipher) (*Service, error) {
	if store == nil {
		return nil, errors.New("secrets store is required")
	}
	if keys == nil {
		return nil, errors.New("private keys are required")
	}
	if cipher == nil {
		return nil, errors.New("cipher is required")
	}
	return &Service{store: store, keys: keys, cipher: cipher}, nil
}

// seal encrypts plaintext under publicKey and wraps it in an envelope.
func (s *Service) seal(ref Reference, plaintext, publicKey string) (string, error) {
	native, err := s.cipher.Encrypt(plaintext, publicKey)
	if err != nil {
		return "", fmt.Errorf(
			"encrypting secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	sealed, err := envelope{algorithm: s.cipher.Algorithm(), payload: native}.encode()
	if err != nil {
		return "", fmt.Errorf(
			"encoding secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return sealed, nil
}

// open decodes and decrypts one stored envelope.
func (s *Service) open(ref Reference, stored, privateKey string) (string, error) {
	payload, err := openEnvelope(ref, stored, s.cipher.Algorithm())
	if err != nil {
		return "", err
	}
	plaintext, err := s.cipher.Decrypt(payload, privateKey)
	if err != nil {
		return "", fmt.Errorf(
			"decrypting secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return plaintext, nil
}

// cloneGroup copies a snapshot's group so it can be edited as a replacement,
// naming a new group when the snapshot does not exist.
func (s *Service) cloneGroup(snap Snapshot, name string) Group {
	group := Group{
		Name:      snap.Group.Name,
		PublicKey: snap.Group.PublicKey,
		Values:    maps.Clone(snap.Group.Values),
	}
	if group.Name == "" {
		group.Name = name
	}
	if group.Values == nil {
		group.Values = make(map[string]string)
	}
	return group
}

// sortedKeys returns a group's value keys in ascending order.
func (s *Service) sortedKeys(values map[string]string) []string {
	return slices.Sorted(maps.Keys(values))
}

// noPublicKey reports a group that needs a keypair before storing secrets.
func (s *Service) noPublicKey(group string) error {
	return fmt.Errorf(
		"group %q has no public key; run 'envx keypair generate %s' first: %w",
		group, group, ErrGroupNotFound,
	)
}
