package env

import (
	"errors"
	"strings"
	"testing"
)

// diffFixture serves two environments whose secrets share a plaintext.
func diffFixture(t *testing.T, host ...string) *Service {
	t.Helper()
	namespaces := newFakeNamespaces().
		base(t, appInclude, `
			host: db.local
			removed: gone
			token: secret://dev/token
		`).
		overlay(t, appInclude, "production", `
			host: prod.local
			removed: ~
			token: secret://prod/token
			added: new
		`)
	opener := &fakeOpener{plaintext: map[string]string{
		"secret://dev/token":  "same",
		"secret://prod/token": "same",
	}}
	return newService(t, testWorkspace(), Dependencies{
		Namespaces:   namespaces,
		HostEnv:      hostOf(host...),
		OpenResolver: opener.open,
	})
}

// changeKeys summarizes changes as key:kind strings.
func changeKeys(changes []Change) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		kind := "changed"
		switch {
		case c.Before == nil:
			kind = "added"
		case c.After == nil:
			kind = "removed"
		}
		out = append(out, c.Key+":"+kind)
	}
	return out
}

// TestDiff verifies masked diffs compare declarations and revealed diffs
// compare resolved values.
func TestDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// reveal compares resolved values.
		reveal bool
		// want lists expected changes.
		want string
	}{
		{want: "ADDED:added HOST:changed REMOVED:changed TOKEN:changed"},
		{reveal: true, want: "ADDED:added HOST:changed REMOVED:changed"},
	}
	for _, tt := range tests {
		result, err := diffFixture(t).Diff(DiffParams{
			From: "development", To: "production", Reveal: tt.reveal,
		})
		if err != nil {
			t.Fatalf("Diff(reveal=%v): %v", tt.reveal, err)
		}
		if got := strings.Join(changeKeys(result.Changes), " "); got != tt.want {
			t.Errorf("Diff(reveal=%v) = %s, want %s", tt.reveal, got, tt.want)
		}
	}
}

// TestDiffEntries verifies changes carry both sides' entries.
func TestDiffEntries(t *testing.T) {
	t.Parallel()

	result, err := diffFixture(t).Diff(DiffParams{To: "production"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	host := result.Changes[1]
	if host.Key != "HOST" || host.Before.Raw != "db.local" ||
		host.After.Raw != "prod.local" {
		t.Errorf("HOST change = %+v", host)
	}
	if !host.Before.DeclaredInBase ||
		host.After.Origin.Winner.Path != appInclude+".production.yaml" {
		t.Errorf("HOST entries = %+v / %+v", host.Before, host.After)
	}
	token := result.Changes[3]
	if !token.Before.Secret || !token.After.Secret {
		t.Errorf("TOKEN change should be secret: %+v", token)
	}
}

// TestDiffHostPinned verifies a host value pinning a key on both sides hides
// the file difference.
func TestDiffHostPinned(t *testing.T) {
	t.Parallel()

	result, err := diffFixture(t, "HOST=pinned").Diff(DiffParams{To: "production"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, c := range result.Changes {
		if c.Key == "HOST" {
			t.Errorf("HOST reported as changed: %+v", c)
		}
	}
}

// TestDiffFailures verifies undeclared environments and unresolved revealed
// values are fatal.
func TestDiffFailures(t *testing.T) {
	t.Parallel()

	s := diffFixture(t)
	_, err := s.Diff(DiffParams{From: "staging", To: "production"})
	if !errors.Is(err, ErrEnvironmentNotDeclared) {
		t.Errorf("undeclared error = %v", err)
	}

	broken := newService(t, testWorkspace(), Dependencies{
		Namespaces:   newFakeNamespaces().base(t, appInclude, "token: secret://x/y"),
		OpenResolver: (&fakeOpener{}).open,
	})
	if _, err := broken.Diff(DiffParams{To: "production"}); err != nil {
		t.Errorf("masked diff with dangling secret: %v", err)
	}
	_, err = broken.Diff(DiffParams{To: "production", Reveal: true})
	if !errors.Is(err, ErrUnresolved) || !strings.Contains(err.Error(), "TOKEN") {
		t.Errorf("revealed dangling error = %v, want ErrUnresolved naming TOKEN", err)
	}
}
