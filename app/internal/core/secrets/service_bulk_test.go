package secrets_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// TestEncryptIsIdempotentAndStagesAllGroups verifies Encrypt seals every
// plaintext value once and leaves existing envelopes alone.
func TestEncryptIsIdempotentAndStagesAllGroups(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")
	f.generate(t, "web")
	f.store.put(secrets.Group{
		Name:      "api",
		PublicKey: f.store.groups["api"].PublicKey,
		Values:    map[string]string{"A": "1", "B": "2"},
	})
	f.store.put(secrets.Group{
		Name:      "web",
		PublicKey: f.store.groups["web"].PublicKey,
		Values:    map[string]string{"C": "3"},
	})

	res, err := f.service.EncryptSecrets("", "")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if len(res.Secrets) != 3 {
		t.Errorf("Secrets = %v, want 3 references", res.Secrets)
	}

	res, err = f.service.EncryptSecrets("", "")
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}
	if len(res.Secrets) != 0 {
		t.Errorf("second Encrypt changed %v, want none", res.Secrets)
	}

	got, err := f.service.GetSecret("web", "C")
	if err != nil || got.Value != "3" {
		t.Errorf("Get after Encrypt = %q, %v", got.Value, err)
	}
}

// TestEncryptFailureCommitsNothing verifies a staging failure in any group
// prevents every commit.
func TestEncryptFailureCommitsNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")
	f.store.put(secrets.Group{
		Name:      "api",
		PublicKey: f.store.groups["api"].PublicKey,
		Values:    map[string]string{"A": "1"},
	})
	f.store.put(secrets.Group{Name: "orphan", Values: map[string]string{"B": "2"}})
	commits := f.store.commits

	_, err := f.service.EncryptSecrets("", "")
	mustErrorIs(t, err, secrets.ErrGroupNotFound)
	if f.store.commits != commits {
		t.Errorf("committed %d groups after failure", f.store.commits-commits)
	}
}

// TestEncryptReportsCommittedGroupsOnFailure verifies a failed later commit
// still reports the groups already committed.
func TestEncryptReportsCommittedGroupsOnFailure(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, group := range []string{"api", "web"} {
		f.generate(t, group)
		f.store.put(secrets.Group{
			Name:      group,
			PublicKey: f.store.groups[group].PublicKey,
			Values:    map[string]string{"K": "v"},
		})
	}
	commits := f.store.commits
	f.store.beforeCommit = func() {
		if f.store.commits > commits {
			f.store.revisions["web"]++
		}
	}

	res, err := f.service.EncryptSecrets("", "")
	mustErrorIs(t, err, secrets.ErrConflict)
	want := []secrets.Reference{{Group: "api", Key: "K"}}
	if !slices.Equal(res.Secrets, want) {
		t.Errorf("Secrets = %v, want %v", res.Secrets, want)
	}
}

// TestDecryptReportsUnavailableGroups verifies groups without a private key are
// skipped and reported while other groups decrypt.
func TestDecryptReportsUnavailableGroups(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")
	f.generate(t, "web")
	f.set(t, "api", "A", "1")
	f.set(t, "web", "B", "2")
	f.set(t, "web", "C", "3")
	delete(f.keys.byPublic, "web")

	res, err := f.service.DecryptSecrets("", "")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	want := []secrets.Reference{{Group: "api", Key: "A"}}
	if !slices.Equal(res.Secrets, want) {
		t.Errorf("Secrets = %v, want %v", res.Secrets, want)
	}
	if !slices.Equal(res.UnavailableGroups, []string{"web"}) {
		t.Errorf("UnavailableGroups = %v, want [web]", res.UnavailableGroups)
	}
	if got := f.stored("api", "A"); got != "1" {
		t.Errorf("decrypted value = %q, want 1", got)
	}
}

// TestSelection verifies selection validation and that explicit selections
// must match a stored value.
func TestSelection(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")
	f.set(t, "api", "A", "1")

	tests := []struct {
		name    string
		group   string
		key     string
		wantErr string
	}{
		{"key without group", "", "A", "requires a group"},
		{"unknown group", "web", "", "no secrets found"},
		{"unknown key", "api", "Z", `no secret "Z"`},
		{"matching key", "API", "A", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.service.EncryptSecrets(tt.group, tt.key)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Encrypt: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
