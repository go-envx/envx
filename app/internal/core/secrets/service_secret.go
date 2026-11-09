package secrets

import (
	"errors"
	"fmt"
)

// GetResult reports a decrypted value and the store it was read from.
type GetResult struct {
	// Value is the decrypted plaintext.
	Value string
	// Location identifies the store that held the value.
	Location string
}

// MutationResult reports the references a mutation changed.
type MutationResult struct {
	// Location identifies the store that was changed.
	Location string
	// Changed lists the references whose stored values were committed.
	Changed []Reference
	// UnavailableGroups lists groups skipped because no private key was found.
	UnavailableGroups []string
}

// Get decrypts one stored secret.
func (s *Service) Get(reference Reference) (GetResult, error) {
	ref, err := reference.normalized()
	if err != nil {
		return GetResult{}, err
	}
	snap, err := s.store.Read(ref.Group)
	if err != nil {
		return GetResult{}, err
	}
	stored, ok := snap.Group.Values[ref.Key]
	if !ok {
		return GetResult{}, notFound(ref)
	}
	payload, err := openEnvelope(ref, stored, s.cipher.Algorithm())
	if err != nil {
		return GetResult{}, err
	}
	key, err := s.keys.Resolve(ref.Group, snap.Group.PublicKey)
	if err != nil {
		return GetResult{}, fmt.Errorf(
			"resolving private key for group %q: %w", ref.Group, err,
		)
	}
	plaintext, err := s.cipher.Decrypt(payload, key.Value)
	if err != nil {
		return GetResult{}, fmt.Errorf(
			"decrypting secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return GetResult{Value: plaintext, Location: s.store.Location()}, nil
}

// Set encrypts plaintext under the group's public key and stores it.
func (s *Service) Set(reference Reference, plaintext string) (MutationResult, error) {
	ref, err := reference.normalized()
	if err != nil {
		return MutationResult{}, err
	}
	if plaintext == "" {
		return MutationResult{}, errors.New("secret plaintext is empty")
	}
	snap, err := s.store.Read(ref.Group)
	if err != nil {
		return MutationResult{}, err
	}
	if snap.Group.PublicKey == "" {
		return MutationResult{}, s.noPublicKey(ref.Group)
	}
	sealed, err := s.seal(ref, plaintext, snap.Group.PublicKey)
	if err != nil {
		return MutationResult{}, err
	}
	replacement := s.cloneGroup(snap, ref.Group)
	replacement.Values[ref.Key] = sealed
	if err := s.store.Commit(snap, replacement); err != nil {
		return MutationResult{}, fmt.Errorf(
			"saving secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return s.mutated([]Reference{ref}, nil), nil
}

// Delete removes one stored secret, keeping the group's public key.
func (s *Service) Delete(reference Reference) (MutationResult, error) {
	ref, err := reference.normalized()
	if err != nil {
		return MutationResult{}, err
	}
	snap, err := s.store.Read(ref.Group)
	if err != nil {
		return MutationResult{}, err
	}
	if _, ok := snap.Group.Values[ref.Key]; !ok {
		return MutationResult{}, notFound(ref)
	}
	replacement := s.cloneGroup(snap, ref.Group)
	delete(replacement.Values, ref.Key)
	if err := s.store.Commit(snap, replacement); err != nil {
		return MutationResult{}, fmt.Errorf(
			"deleting secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return s.mutated([]Reference{ref}, nil), nil
}
