package secrets

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/lib/crypt"
)

// RotateKeypair replaces a group's identity and re-encrypts all of its values.
// The new private key is retained, and verified not to be shadowed, before the
// new public key and ciphertext are committed together. The old private key
// is left in place.
func (s *Service) RotateKeypair(group string) (KeypairResult, error) {
	group, err := NormalizeGroup(group)
	if err != nil {
		return KeypairResult{}, err
	}
	snap, err := s.store.Read(group)
	if err != nil {
		return KeypairResult{}, err
	}
	if snap.Group.PublicKey == "" {
		return KeypairResult{}, s.noPublicKey(group)
	}
	oldKey, err := s.keys.Resolve(group, snap.Group.PublicKey)
	if errors.Is(err, ErrKeyNotAvailable) {
		return KeypairResult{}, fmt.Errorf(
			"cannot rotate group %q: its private key is not available", group,
		)
	}
	if err != nil {
		return KeypairResult{}, fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}
	if oldKey.Origin != OriginStore {
		return KeypairResult{}, s.rotationShadowed(group, oldKey.Origin)
	}
	plaintexts, err := s.decryptForRotation(group, snap.Group.Values, oldKey.Value)
	if err != nil {
		return KeypairResult{}, err
	}

	pair, err := s.newKeypair(group)
	if err != nil {
		return KeypairResult{}, err
	}
	replacement := Group{
		Name:      snap.Group.Name,
		PublicKey: pair.PublicKey,
		Values:    make(map[string]string, len(plaintexts)),
	}
	refs := make([]Reference, 0, len(plaintexts))
	for _, key := range s.sortedKeys(plaintexts) {
		ref := Reference{Group: group, Key: key}
		sealed, err := s.seal(ref, plaintexts[key], pair.PublicKey)
		if err != nil {
			return KeypairResult{}, err
		}
		replacement.Values[key] = sealed
		refs = append(refs, ref)
	}

	if err := s.retainForRotation(group, pair); err != nil {
		return KeypairResult{}, err
	}
	if err := s.store.Commit(snap, replacement); err != nil {
		return KeypairResult{}, fmt.Errorf(
			"committing rotated keypair for group %q: %w", group, err,
		)
	}
	return s.keypairResult(group, pair.PublicKey, refs), nil
}

// decryptForRotation decrypts every value in a group, requiring each to be encrypted.
func (s *Service) decryptForRotation(
	group string, values map[string]string, privateKey string,
) (map[string]string, error) {
	plaintexts := make(map[string]string, len(values))
	for _, key := range s.sortedKeys(values) {
		ref := Reference{Group: group, Key: key}
		if !isEnvelope(values[key]) {
			return nil, fmt.Errorf(
				"secret %q in group %q is not encrypted; "+
					"encrypt the group before rotating",
				key, group,
			)
		}
		plaintext, err := s.open(ref, values[key], privateKey)
		if err != nil {
			return nil, err
		}
		plaintexts[key] = plaintext
	}
	return plaintexts, nil
}

// retainForRotation retains the new private key and rejects a higher-priority
// key source that would shadow it after the public key is committed.
func (s *Service) retainForRotation(group string, pair crypt.Keypair) error {
	if err := s.keys.Retain(group, pair); err != nil {
		return fmt.Errorf("writing private key for group %q: %w", group, err)
	}
	resolved, err := s.keys.Resolve(group, pair.PublicKey)
	if err != nil {
		return fmt.Errorf(
			"verifying private key for group %q: %w", group, err,
		)
	}
	if resolved.Value != pair.PrivateKey {
		return s.rotationShadowed(group, resolved.Origin)
	}
	return nil
}

// rotationShadowed reports a key source that would hide a rotated private key.
func (s *Service) rotationShadowed(group, origin string) error {
	return fmt.Errorf(
		"cannot rotate group %q: its private key resolves from %s, which "+
			"has higher lookup priority and would shadow the new key",
		group, origin,
	)
}
