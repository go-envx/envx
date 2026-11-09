package secrets

import (
	"errors"
	"fmt"
	"strings"
)

// transform returns a selected value's replacement and whether it changed.
type transform func(snap *Snapshot, ref Reference, stored string) (string, bool, error)

// Selection narrows bulk operations. Empty selects every group and key.
type Selection struct {
	// Group limits the operation to one group; empty selects all groups.
	Group string
	// Key limits the operation to one key and requires Group.
	Key string
}

// normalized validates s and returns it with a lowercase group.
func (s Selection) normalized() (Selection, error) {
	if s.Group != "" {
		group, err := NormalizeGroup(s.Group)
		if err != nil {
			return Selection{}, err
		}
		s.Group = group
	}
	if s.Key != "" {
		if s.Group == "" {
			return Selection{}, errors.New("selecting a secret key requires a group")
		}
		if err := ValidateKey(s.Key); err != nil {
			return Selection{}, err
		}
	}
	return s, nil
}

// explicit reports whether the selection narrows the store, so it must match.
func (s Selection) explicit() bool {
	return s.Group != ""
}

// noMatch reports an explicit selection that matched no stored value.
func (s Selection) noMatch() error {
	if s.Key != "" {
		return fmt.Errorf("no secret %q found in group %q", s.Key, s.Group)
	}
	return fmt.Errorf("no secrets found in group %q", s.Group)
}

// Encrypt encrypts selected plaintext values in place. Values that are already
// encrypted are skipped, and every group is staged before any commit.
func (s *Service) Encrypt(selection Selection) (MutationResult, error) {
	return s.bulk(selection, func(
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
}

// Decrypt decrypts selected ciphertext values in place. Groups without an
// available private key are skipped and reported as unavailable.
func (s *Service) Decrypt(selection Selection) (MutationResult, error) {
	keys := make(map[string]string)
	var unavailable []string
	result, err := s.bulk(selection, func(
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
	result.UnavailableGroups = unavailable
	return result, err
}

// bulk applies apply to every selected value, staging all groups in memory
// before committing any of them. Each group commits atomically; when a later
// commit fails, the result still reports the references already committed.
// An explicit selection must match a value.
func (s *Service) bulk(selection Selection, apply transform) (MutationResult, error) {
	sel, err := selection.normalized()
	if err != nil {
		return MutationResult{}, err
	}
	snaps, err := s.store.List()
	if err != nil {
		return MutationResult{}, err
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
		if sel.Group != "" && sel.Group != group {
			continue
		}
		replacement := s.cloneGroup(*snap, group)
		var refs []Reference
		for _, key := range s.sortedKeys(snap.Group.Values) {
			if sel.Key != "" && sel.Key != key {
				continue
			}
			matched = true
			ref := Reference{Group: group, Key: key}
			updated, ok, err := apply(snap, ref, snap.Group.Values[key])
			if err != nil {
				return MutationResult{}, err
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
		return MutationResult{}, sel.noMatch()
	}

	var changed []Reference
	for i := range commits {
		c := &commits[i]
		if err := s.store.Commit(c.expected, c.replacement); err != nil {
			return s.mutated(changed, nil), fmt.Errorf(
				"saving secrets for group %q: %w", c.replacement.Name, err,
			)
		}
		changed = append(changed, c.refs...)
	}
	return s.mutated(changed, nil), nil
}
