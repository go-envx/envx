package secrets

import (
	"path/filepath"
	"testing"
)

// TestParseReference verifies reference parsing classifies plain values, escaped
// literals, and malformed references as non-references and lowercases the group.
func TestParseReference(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		value     string
		wantGroup string
		wantKey   string
		wantOK    bool
	}{
		{
			name: "reference", value: "secret://production/api_key",
			wantGroup: "production", wantKey: "api_key", wantOK: true,
		},
		{
			name: "group lowercased", value: "secret://Production/api_key",
			wantGroup: "production", wantKey: "api_key", wantOK: true,
		},
		{name: "plain value", value: "postgres://localhost", wantOK: false},
		{name: "escaped literal", value: `\secret://production/api_key`, wantOK: false},
		{name: "missing key", value: "secret://production", wantOK: false},
		{name: "empty reference", value: "secret://", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			group, key, ok := ParseReference(tc.value)
			if ok != tc.wantOK {
				t.Fatalf("ParseReference(%q) ok = %v, want %v", tc.value, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if group != tc.wantGroup || key != tc.wantKey {
				t.Errorf(
					"ParseReference(%q) = %q/%q, want %q/%q",
					tc.value, group, key, tc.wantGroup, tc.wantKey,
				)
			}
		})
	}
}

// TestStoredSecretsReportsEncryptionState verifies StoredSecrets classifies each
// stored value's encryption state without exposing the value.
func TestStoredSecretsReportsEncryptionState(t *testing.T) {
	t.Parallel()

	// "YWJj" is the base64url payload for "abc"; a plaintext value has no envelope.
	storePath := writeStore(t, "secrets:\n"+
		"  production:\n"+
		"    enc: encrypted-age:YWJj\n"+
		"    plain: just-plaintext\n")
	manager := newInventoryManager(t, storePath)

	stored, err := manager.StoredSecrets()
	if err != nil {
		t.Fatalf("StoredSecrets(): %v", err)
	}
	got := map[string]bool{}
	for _, s := range stored {
		got[s.Group+"/"+s.Key] = s.Encrypted
	}
	if enc, ok := got["production/enc"]; !ok || !enc {
		t.Errorf("production/enc encrypted = %v (present=%v), want true", enc, ok)
	}
	if enc, ok := got["production/plain"]; !ok || enc {
		t.Errorf("production/plain encrypted = %v (present=%v), want false", enc, ok)
	}
}

// TestStoredSecretsReportsAlgorithmMismatch verifies an encrypted value whose
// envelope algorithm differs from the configured cipher is flagged, while a value
// under the configured algorithm and a plaintext value are not.
func TestStoredSecretsReportsAlgorithmMismatch(t *testing.T) {
	t.Parallel()

	// The manager's cipher is age; "encrypted-nacl-box:YWJj" is a well-formed
	// envelope under a different algorithm, so it is a mismatch.
	storePath := writeStore(t, "secrets:\n"+
		"  production:\n"+
		"    match: encrypted-age:YWJj\n"+
		"    mismatch: encrypted-nacl-box:YWJj\n"+
		"    plain: just-plaintext\n")
	manager := newInventoryManager(t, storePath)

	stored, err := manager.StoredSecrets()
	if err != nil {
		t.Fatalf("StoredSecrets(): %v", err)
	}
	got := map[string]StoredSecret{}
	for _, s := range stored {
		got[s.Group+"/"+s.Key] = s
	}
	if s := got["production/match"]; !s.Encrypted || s.AlgorithmMismatch {
		t.Errorf("match = %+v, want encrypted with no mismatch", s)
	}
	if s := got["production/mismatch"]; !s.Encrypted || !s.AlgorithmMismatch {
		t.Errorf("mismatch = %+v, want encrypted with a mismatch", s)
	}
	if s := got["production/plain"]; s.Encrypted || s.AlgorithmMismatch {
		t.Errorf("plain = %+v, want plaintext with no mismatch", s)
	}
}

// TestGroupsMissingPublicKey verifies only groups that hold stored secrets but no
// public key are reported, each once and in document order.
func TestGroupsMissingPublicKey(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public_keys:\n"+
		"  known: age1examplepublickeyvalue\n"+
		"secrets:\n"+
		"  known:\n"+
		"    a: encrypted-age:YWJj\n"+
		"  orphan:\n"+
		"    b: encrypted-age:YWJj\n"+
		"    c: encrypted-age:YWJj\n")
	manager := newInventoryManager(t, storePath)

	groups, err := manager.GroupsMissingPublicKey()
	if err != nil {
		t.Fatalf("GroupsMissingPublicKey(): %v", err)
	}
	if len(groups) != 1 || groups[0] != "orphan" {
		t.Errorf("GroupsMissingPublicKey() = %v, want [orphan]", groups)
	}
}

// TestStoredSecretsMissingStoreIsEmpty verifies a missing store yields no entries
// rather than an error.
func TestStoredSecretsMissingStoreIsEmpty(t *testing.T) {
	t.Parallel()

	manager := newInventoryManager(t, filepath.Join(t.TempDir(), "absent.yaml"))
	stored, err := manager.StoredSecrets()
	if err != nil {
		t.Fatalf("StoredSecrets(): %v", err)
	}
	if len(stored) != 0 {
		t.Errorf("StoredSecrets() = %d entries, want 0", len(stored))
	}
}

// TestKeypairsReportsAvailableStatus verifies Keypairs enumerates every group
// with a public key and reports a valid available key and an unavailable one.
func TestKeypairsReportsAvailableStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.yaml")
	keysPath := filepath.Join(dir, "envx.keys")
	manager := newLocalKeypairManager(t, storePath, keysPath)

	// A generated group has its private key available and valid.
	if _, err := manager.GenerateKeypair("available"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}

	keypairs, err := manager.Keypairs()
	if err != nil {
		t.Fatalf("Keypairs(): %v", err)
	}
	if len(keypairs) != 1 {
		t.Fatalf("Keypairs() = %d groups, want 1", len(keypairs))
	}
	if keypairs[0].Group != "available" {
		t.Errorf("group = %q, want available", keypairs[0].Group)
	}
	if keypairs[0].PrivateKeyStatus != PrivateKeyValid {
		t.Errorf("status = %q, want valid", keypairs[0].PrivateKeyStatus)
	}
}

// TestKeypairsReportsUnavailableKey verifies a group whose public key is present
// but whose private key cannot be resolved is reported as not available.
func TestKeypairsReportsUnavailableKey(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public_keys:\n  orphaned: age1examplepublickeyvalue\n")
	manager := newInventoryManager(t, storePath)

	keypairs, err := manager.Keypairs()
	if err != nil {
		t.Fatalf("Keypairs(): %v", err)
	}
	if len(keypairs) != 1 {
		t.Fatalf("Keypairs() = %d groups, want 1", len(keypairs))
	}
	if keypairs[0].PrivateKeyStatus != PrivateKeyNotAvailable {
		t.Errorf("status = %q, want not_available", keypairs[0].PrivateKeyStatus)
	}
}

// newInventoryManager builds a manager over storePath with a no-op private-key
// resolver, for inventory tests that never need to resolve key material.
func newInventoryManager(t *testing.T, storePath string) *Manager {
	t.Helper()
	manager, err := New(Params{
		SecretsPath:       storePath,
		KeysPath:          filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:     2,
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return manager
}
