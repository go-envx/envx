package env

import (
	"errors"
	"slices"
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
		EnvNamespaces:    namespaces,
		HostEnv:          hostOf(host...),
		NewValueResolver: opener.open,
	})
}

// changeKeys summarizes a diff as sorted key:kind strings.
func changeKeys(result DiffResult) string {
	var out []string
	for kind, changes := range map[string][]Change{
		"added":   result.Added,
		"removed": result.Removed,
		"changed": result.Changed,
	} {
		for _, c := range changes {
			out = append(out, c.Key+":"+kind)
		}
	}
	slices.Sort(out)
	return strings.Join(out, " ")
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
			EnvironmentA: "development", EnvironmentB: "production", Reveal: tt.reveal,
		})
		if err != nil {
			t.Fatalf("Diff(reveal=%v): %v", tt.reveal, err)
		}
		if got := changeKeys(result); got != tt.want {
			t.Errorf("Diff(reveal=%v) = %s, want %s", tt.reveal, got, tt.want)
		}
	}
}

// TestDiffValues verifies changes carry each side's compared text: declared
// literals when masked.
func TestDiffValues(t *testing.T) {
	t.Parallel()

	result, err := diffFixture(t).Diff(DiffParams{EnvironmentB: "production"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	byKey := make(map[string]Change)
	for _, c := range result.Changed {
		byKey[c.Key] = c
	}
	if host := byKey["HOST"]; host.Before != "db.local" || host.After != "prod.local" {
		t.Errorf("HOST change = %+v", host)
	}
	token := byKey["TOKEN"]
	if token.Before != "secret://dev/token" || token.After != "secret://prod/token" {
		t.Errorf("TOKEN change = %+v", token)
	}
	if len(result.Added) != 1 || result.Added[0].After != "new" {
		t.Errorf("Added = %+v", result.Added)
	}
}

// TestDiffHostPinned verifies a host value pinning a key on both sides hides
// the file difference.
func TestDiffHostPinned(t *testing.T) {
	t.Parallel()

	result, err := diffFixture(t, "HOST=pinned").Diff(
		DiffParams{EnvironmentB: "production"},
	)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, c := range result.Changed {
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
	_, err := s.Diff(DiffParams{EnvironmentA: "staging", EnvironmentB: "production"})
	if !errors.Is(err, ErrEnvironmentNotDeclared) {
		t.Errorf("undeclared error = %v", err)
	}

	broken := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    newFakeNamespaces().base(t, appInclude, "token: secret://x/y"),
		NewValueResolver: (&fakeOpener{}).open,
	})
	if _, err := broken.Diff(DiffParams{EnvironmentB: "production"}); err != nil {
		t.Errorf("masked diff with dangling secret: %v", err)
	}
	_, err = broken.Diff(DiffParams{EnvironmentB: "production", Reveal: true})
	if !errors.Is(err, ErrUnresolved) || !strings.Contains(err.Error(), "TOKEN") {
		t.Errorf("revealed dangling error = %v, want ErrUnresolved naming TOKEN", err)
	}
}
