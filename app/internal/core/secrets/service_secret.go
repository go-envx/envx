package secrets

import (
	"errors"
	"fmt"
)

// GetSecretResult reports a decrypted value and the store it was read from.
type GetSecretResult struct {
	// Value is the decrypted plaintext.
	Value string
	// Location identifies the store that held the value.
	Location string
}

// SetSecretResult reports a stored secret and the store that holds it.
type SetSecretResult struct {
	// Location identifies the store that was changed.
	Location string
	// Secret identifies the stored secret.
	Secret Reference
}

// DeleteSecretResult reports a removed secret and the store it was removed from.
type DeleteSecretResult struct {
	// Location identifies the store that was changed.
	Location string
	// Secret identifies the removed secret.
	Secret Reference
}

// GetSecret decrypts one stored secret.
func (s *Service) GetSecret(group, key string) (GetSecretResult, error) {
	ref, err := Reference{Group: group, Key: key}.normalized()
	if err != nil {
		return GetSecretResult{}, err
	}
	snap, err := s.store.Read(ref.Group)
	if err != nil {
		return GetSecretResult{}, err
	}
	stored, ok := snap.Group.Values[ref.Key]
	if !ok {
		return GetSecretResult{}, notFound(ref)
	}
	payload, err := openEnvelope(ref, stored, s.cipher.Algorithm())
	if err != nil {
		return GetSecretResult{}, err
	}
	privateKey, err := s.keys.Resolve(ref.Group, snap.Group.PublicKey)
	if err != nil {
		return GetSecretResult{}, fmt.Errorf(
			"resolving private key for group %q: %w", ref.Group, err,
		)
	}
	plaintext, err := s.cipher.Decrypt(payload, privateKey.Value)
	if err != nil {
		return GetSecretResult{}, fmt.Errorf(
			"decrypting secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return GetSecretResult{Value: plaintext, Location: s.store.Location()}, nil
}

// SetSecret encrypts a plaintext value under the group's public key and stores
// it. The plaintext source is called only after the reference is valid and the
// group has a public key, so a caller never prompts for a value it cannot store.
func (s *Service) SetSecret(
	group, key string, plaintext func() (string, error),
) (SetSecretResult, error) {
	ref, err := Reference{Group: group, Key: key}.normalized()
	if err != nil {
		return SetSecretResult{}, err
	}
	if plaintext == nil {
		return SetSecretResult{}, errors.New("plaintext source is nil")
	}
	snap, err := s.store.Read(ref.Group)
	if err != nil {
		return SetSecretResult{}, err
	}
	if snap.Group.PublicKey == "" {
		return SetSecretResult{}, s.noPublicKey(ref.Group)
	}
	value, err := plaintext()
	if err != nil {
		return SetSecretResult{}, err
	}
	if value == "" {
		return SetSecretResult{}, errors.New("secret plaintext is empty")
	}
	sealed, err := s.seal(ref, value, snap.Group.PublicKey)
	if err != nil {
		return SetSecretResult{}, err
	}
	replacement := s.cloneGroup(snap, ref.Group)
	replacement.Values[ref.Key] = sealed
	if err := s.store.Commit(snap, replacement); err != nil {
		return SetSecretResult{}, fmt.Errorf(
			"saving secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return SetSecretResult{Location: s.store.Location(), Secret: ref}, nil
}

// DeleteSecret removes one stored secret, keeping the group's public key.
func (s *Service) DeleteSecret(group, key string) (DeleteSecretResult, error) {
	ref, err := Reference{Group: group, Key: key}.normalized()
	if err != nil {
		return DeleteSecretResult{}, err
	}
	snap, err := s.store.Read(ref.Group)
	if err != nil {
		return DeleteSecretResult{}, err
	}
	if _, ok := snap.Group.Values[ref.Key]; !ok {
		return DeleteSecretResult{}, notFound(ref)
	}
	replacement := s.cloneGroup(snap, ref.Group)
	delete(replacement.Values, ref.Key)
	if err := s.store.Commit(snap, replacement); err != nil {
		return DeleteSecretResult{}, fmt.Errorf(
			"deleting secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return DeleteSecretResult{Location: s.store.Location(), Secret: ref}, nil
}
