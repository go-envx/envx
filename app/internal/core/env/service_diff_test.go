package env

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// diffFixture serves two environments whose secrets share a plaintext; opener
// records any resolver opened.
func diffFixture(t *testing.T, host ...string) (*Service, *fakeOpener) {
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
	}), opener
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

// TestDiff verifies diffs compare declarations without opening a resolver.
func TestDiff(t *testing.T) {
	t.Parallel()

	s, opener := diffFixture(t)
	result, err := s.Diff(DiffParams{
		EnvironmentA: "development", EnvironmentB: "production",
	})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	want := "ADDED:added HOST:changed REMOVED:changed TOKEN:changed"
	if got := changeKeys(result); got != want {
		t.Errorf("Diff = %s, want %s", got, want)
	}
	if opener.calls != 0 {
		t.Errorf("resolver opened %d times, want 0", opener.calls)
	}
}

// TestDiffValues verifies changes carry each side's declared literal.
func TestDiffValues(t *testing.T) {
	t.Parallel()

	s, _ := diffFixture(t)
	result, err := s.Diff(DiffParams{EnvironmentB: "production"})
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

	s, _ := diffFixture(t, "HOST=pinned")
	result, err := s.Diff(DiffParams{EnvironmentB: "production"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, c := range result.Changed {
		if c.Key == "HOST" {
			t.Errorf("HOST reported as changed: %+v", c)
		}
	}
}

// TestDiffFailures verifies undeclared environments are fatal and dangling
// secrets are not.
func TestDiffFailures(t *testing.T) {
	t.Parallel()

	s, _ := diffFixture(t)
	_, err := s.Diff(DiffParams{EnvironmentA: "staging", EnvironmentB: "production"})
	if !errors.Is(err, ErrEnvironmentNotDeclared) {
		t.Errorf("undeclared error = %v", err)
	}

	broken := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    newFakeNamespaces().base(t, appInclude, "token: secret://x/y"),
		NewValueResolver: (&fakeOpener{}).open,
	})
	if _, err := broken.Diff(DiffParams{EnvironmentB: "production"}); err != nil {
		t.Errorf("diff with dangling secret: %v", err)
	}
}
