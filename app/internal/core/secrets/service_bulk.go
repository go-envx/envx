package secrets

import (
	"errors"
	"fmt"
	"strings"
)

// EncryptSecretsResult reports the values encrypted in place.
type EncryptSecretsResult struct {
	// Location identifies the store that was changed.
	Location string
	// Secrets lists the references whose values were encrypted.
	Secrets []Reference
}

// DecryptSecretsResult reports the values decrypted in place and the groups
// skipped for lack of a private key.
type DecryptSecretsResult struct {
	// Location identifies the store that was changed.
	Location string
	// Secrets lists the references whose values were decrypted.
	Secrets []Reference
	// UnavailableGroups lists groups skipped because no private key was found.
	UnavailableGroups []string
}

// transform returns a selected value's replacement and whether it changed.
type transform func(snap *Snapshot, ref Reference, stored string) (string, bool, error)

// selection narrows bulk operations. Empty selects every group and key.
type selection struct {
	// group limits the operation to one group; empty selects all groups.
	group string
	// key limits the operation to one key and requires group.
	key string
}

// normalized validates s and returns it with a lowercase group.
func (s selection) normalized() (selection, error) {
	if s.group != "" {
		group, err := NormalizeGroup(s.group)
		if err != nil {
			return selection{}, err
		}
		s.group = group
	}
	if s.key != "" {
		if s.group == "" {
			return selection{}, errors.New("selecting a secret key requires a group")
		}
		if err := ValidateKey(s.key); err != nil {
			return selection{}, err
		}
	}
	return s, nil
}

// explicit reports whether the selection narrows the store, so it must match.
func (s selection) explicit() bool {
	return s.group != ""
}

// noMatch reports an explicit selection that matched no stored value.
func (s selection) noMatch() error {
	if s.key != "" {
		return fmt.Errorf("no secret %q found in group %q", s.key, s.group)
	}
	return fmt.Errorf("no secrets found in group %q", s.group)
}

// EncryptSecrets encrypts plaintext values in place. An empty group selects
// every group; a key requires a group. Values that are already encrypted are
// skipped, and every group is staged before any commit.
func (s *Service) EncryptSecrets(group, key string) (EncryptSecretsResult, error) {
	changed, err := s.bulk(selection{group: group, key: key}, func(
		snap *Snapshot, ref Reference, stored string,
	) (string, bool, error) {
		if isEnvelope(stored) {
			return "", false, nil
		}
		if snap.Group.PublicKey == "" {
			return "", false, s.noPublicKey(ref.Group)
		}
		sealed, err := s.seal(ref, stored, snap.Group.PublicKey)
		return sealed, err == nil, err
	})
	return EncryptSecretsResult{Location: s.store.Location(), Secrets: changed}, err
}

// DecryptSecrets decrypts ciphertext values in place. An empty group selects
// every group; a key requires a group. Groups without an available private key
// are skipped and reported as unavailable.
func (s *Service) DecryptSecrets(group, key string) (DecryptSecretsResult, error) {
	keys := make(map[string]string)
	var unavailable []string
	changed, err := s.bulk(selection{group: group, key: key}, func(
		snap *Snapshot, ref Reference, stored string,
	) (string, bool, error) {
		if !isEnvelope(stored) {
			return "", false, nil
		}
		key, resolved := keys[ref.Group]
		if !resolved {
			privateKey, err := s.keys.Resolve(ref.Group, snap.Group.PublicKey)
			switch {
			case errors.Is(err, ErrKeyNotAvailable):
				unavailable = append(unavailable, ref.Group)
			case err != nil:
				return "", false, fmt.Errorf(
					"resolving private key for group %q: %w", ref.Group, err,
				)
			}
			key = privateKey.Value
			keys[ref.Group] = key
		}
		if key == "" {
			return "", false, nil
		}
		plaintext, err := s.open(ref, stored, key)
		return plaintext, err == nil, err
	})
	return DecryptSecretsResult{
		Location:          s.store.Location(),
		Secrets:           changed,
		UnavailableGroups: unavailable,
	}, err
}

// bulk applies apply to every selected value, staging all groups in memory
// before committing any of them. Each group commits atomically; when a later
// commit fails, the references already committed are still returned.
// An explicit selection must match a value.
func (s *Service) bulk(sel selection, apply transform) ([]Reference, error) {
	sel, err := sel.normalized()
	if err != nil {
		return nil, err
	}
	snaps, err := s.store.List()
	if err != nil {
		return nil, err
	}

	type commit struct {
		// expected is the snapshot the replacement was staged from.
		expected Snapshot
		// replacement is the staged group to commit.
		replacement Group
		// refs lists the references the replacement changes.
		refs []Reference
	}
	var commits []commit
	matched := false
	for i := range snaps {
		snap := &snaps[i]
		group := strings.ToLower(snap.Group.Name)
		if sel.group != "" && sel.group != group {
			continue
		}
		replacement := s.cloneGroup(*snap, group)
		var refs []Reference
		for _, key := range s.sortedKeys(snap.Group.Values) {
			if sel.key != "" && sel.key != key {
				continue
			}
			matched = true
			ref := Reference{Group: group, Key: key}
			updated, ok, err := apply(snap, ref, snap.Group.Values[key])
			if err != nil {
				return nil, err
			}
			if ok {
				replacement.Values[key] = updated
				refs = append(refs, ref)
			}
		}
		if len(refs) > 0 {
			commits = append(commits, commit{*snap, replacement, refs})
		}
	}
	if sel.explicit() && !matched {
		return nil, sel.noMatch()
	}

	var changed []Reference
	for i := range commits {
		c := &commits[i]
		if err := s.store.Commit(c.expected, c.replacement); err != nil {
			return changed, fmt.Errorf(
				"saving secrets for group %q: %w", c.replacement.Name, err,
			)
		}
		changed = append(changed, c.refs...)
	}
	return changed, nil
}
