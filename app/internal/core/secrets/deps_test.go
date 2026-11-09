package secrets_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/lib/crypt"
)

// memStore is an in-memory secrets.Store with integer revisions.
type memStore struct {
	// groups holds stored groups by lowercase name.
	groups map[string]secrets.Group
	// revisions counts writes per group.
	revisions map[string]int
	// commits counts successful commits.
	commits int
	// beforeCommit runs inside Commit before the revision check.
	beforeCommit func()
	// readErr, when set, fails every Read.
	readErr error
}

// memKeys indexes retained private keys by group and public key. Overrides
// model higher-priority environment keys.
type memKeys struct {
	// byPublic maps group, then public key, to the retained private key.
	byPublic map[string]map[string]string
	// overrides model environment keys that win over retained keys.
	overrides map[string]secrets.PrivateKey
	// resolves counts Resolve calls.
	resolves int
	// retainErr, when set, fails every Retain.
	retainErr error
}

// newMemStore returns a store seeded with groups.
func newMemStore(groups ...secrets.Group) *memStore {
	s := &memStore{
		groups:    make(map[string]secrets.Group),
		revisions: make(map[string]int),
	}
	for _, group := range groups {
		s.put(group)
	}
	return s
}

// newMemKeys returns an empty keyring.
func newMemKeys() *memKeys {
	return &memKeys{
		byPublic:  make(map[string]map[string]string),
		overrides: make(map[string]secrets.PrivateKey),
	}
}

// Location implements secrets.Store.
func (s *memStore) Location() string { return "mem://secrets" }

// Read implements secrets.Reader, returning copies of stored values.
func (s *memStore) Read(group string) (secrets.Snapshot, error) {
	if s.readErr != nil {
		return secrets.Snapshot{}, s.readErr
	}
	name := strings.ToLower(group)
	stored, ok := s.groups[name]
	if !ok {
		return secrets.Snapshot{}, nil
	}
	stored.Values = maps.Clone(stored.Values)
	return secrets.Snapshot{
		Group:    stored,
		Revision: fmt.Sprint(s.revisions[name]),
		Exists:   true,
	}, nil
}

// List implements secrets.Store in group-name order.
func (s *memStore) List() ([]secrets.Snapshot, error) {
	var out []secrets.Snapshot
	for _, name := range slices.Sorted(maps.Keys(s.groups)) {
		snap, _ := s.Read(name)
		out = append(out, snap)
	}
	return out, nil
}

// Commit implements secrets.Store with revision checking.
func (s *memStore) Commit(expected secrets.Snapshot, replacement secrets.Group) error {
	if s.beforeCommit != nil {
		s.beforeCommit()
	}
	current, _ := s.Read(replacement.Name)
	if current.Exists != expected.Exists || current.Revision != expected.Revision {
		return secrets.ErrConflict
	}
	s.commits++
	s.put(replacement)
	return nil
}

// put stores a copy of group and advances its revision.
func (s *memStore) put(group secrets.Group) {
	name := strings.ToLower(group.Name)
	group.Values = maps.Clone(group.Values)
	s.groups[name] = group
	s.revisions[name]++
}

// Location implements secrets.Keys.
func (k *memKeys) Location() string { return "mem://keys" }

// Resolve implements secrets.KeySource, preferring overrides.
func (k *memKeys) Resolve(group, publicKey string) (secrets.PrivateKey, error) {
	k.resolves++
	if key, ok := k.overrides[group]; ok {
		return key, nil
	}
	if key, ok := k.byPublic[group][publicKey]; ok {
		return secrets.PrivateKey{Value: key, Origin: secrets.OriginStore}, nil
	}
	return secrets.PrivateKey{}, secrets.ErrKeyNotAvailable
}

// Retain implements secrets.Keys.
func (k *memKeys) Retain(group string, pair crypt.Keypair) error {
	if k.retainErr != nil {
		return k.retainErr
	}
	if k.byPublic[group] == nil {
		k.byPublic[group] = make(map[string]string)
	}
	k.byPublic[group][pair.PublicKey] = pair.PrivateKey
	return nil
}
