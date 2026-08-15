package envmerge

import (
	"path/filepath"
	"strings"
	"testing"
)

// getManager builds a Manager over a single "app" namespace in dir with the given
// resolver factory (nil for identity behavior).
func getManager(t *testing.T, dir string, factory ValueResolverFactory) *Manager {
	t.Helper()
	return managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})
}

// TestGetReturnsCanonicalKeyAndProvenance verifies a case-insensitive lookup
// returns the canonical uppercase key, the rendered value, and provenance.
func TestGetReturnsCanonicalKeyAndProvenance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\n")

	entry, err := getManager(t, dir, nil).Get(GetParams{Key: "host"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Key != "HOST" {
		t.Errorf("Key = %q, want HOST", entry.Key)
	}
	if entry.Value != "localhost" {
		t.Errorf("Value = %q, want localhost", entry.Value)
	}
	if filepath.Base(entry.Origin.Winner.File) != "app.yaml" {
		t.Errorf("Winner.File = %q, want app.yaml", entry.Origin.Winner.File)
	}
}

// TestGetEnvironment verifies an empty environment uses the configured default,
// an explicit environment is validated and applied per call, and an undeclared
// environment errors.
func TestGetEnvironment(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: base\n")
	writeYAML(t, dir, "app.production.yaml", "host: prod\n")

	manager := managerFor(t, Params{
		Includes:           []string{filepath.Join(dir, "app")},
		DefaultEnvironment: "production",
	})

	t.Run("empty uses configured default", func(t *testing.T) {
		t.Parallel()
		entry, err := manager.Get(GetParams{Key: "host"})
		if err != nil || entry.Value != "prod" {
			t.Fatalf("Get = %q, %v; want prod", entry.Value, err)
		}
	})
	t.Run("explicit environment supersedes default", func(t *testing.T) {
		t.Parallel()
		entry, err := manager.Get(GetParams{Key: "host", Environment: "development"})
		if err != nil || entry.Value != "base" {
			t.Fatalf("Get = %q, %v; want base", entry.Value, err)
		}
	})
	t.Run("undeclared environment errors", func(t *testing.T) {
		t.Parallel()
		if _, err := manager.Get(GetParams{Key: "host", Environment: "ghost"}); err == nil {
			t.Error("expected error for undeclared environment")
		}
	})
}

// TestGetOpensOneResolverWithReveal verifies Get opens exactly one resolver and
// passes the call's reveal policy to the factory.
func TestGetOpensOneResolverWithReveal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "password: secret://x\n")

	factory := &recordingFactory{
		resolver: fakeResolver{values: map[string]string{"secret://x": "pw"}},
	}
	entry, err := getManager(t, dir, factory).Get(GetParams{Key: "password", Reveal: true})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Value != "pw" {
		t.Errorf("Value = %q, want pw", entry.Value)
	}
	if factory.calls != 1 {
		t.Errorf("opened %d resolver(s), want exactly 1", factory.calls)
	}
	if !factory.reveal {
		t.Error("Get did not pass Reveal to the factory")
	}
}

// TestGetResolvesOnlyRequestedWinner verifies an unrelated dangling reference does
// not block the requested key, proving only its winning leaf reaches the resolver.
func TestGetResolvesOnlyRequestedWinner(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "good: secret://ok\nbad: secret://missing\n")

	factory := &recordingFactory{resolver: fakeResolver{
		values:  map[string]string{"secret://ok": "value"},
		failAll: true,
	}}
	entry, err := getManager(t, dir, factory).Get(GetParams{Key: "good", Reveal: true})
	if err != nil {
		t.Fatalf("Get(good): %v", err)
	}
	if entry.Value != "value" {
		t.Errorf("Value = %q, want value", entry.Value)
	}
}

// TestGetReportsRequestedKeyFailure verifies the requested key's own resolution
// failure is surfaced rather than reported as merely absent.
func TestGetReportsRequestedKeyFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "bad: secret://missing\n")

	factory := &recordingFactory{resolver: fakeResolver{failAll: true}}
	_, err := getManager(t, dir, factory).Get(GetParams{Key: "bad", Reveal: true})
	if err == nil {
		t.Fatal("expected the requested key's resolution failure")
	}
}

// TestGetMissingKey verifies an unknown key returns the established not-found
// error.
func TestGetMissingKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\n")

	_, err := getManager(t, dir, nil).Get(GetParams{Key: "nope"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want a not-found message", err)
	}
}

// TestGetListRenderFailureHidesValue verifies a list-render failure is returned
// without leaking the offending item's value.
func TestGetListRenderFailureHidesValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "tokens:\n  - a,b\n  - c\n")

	_, err := getManager(t, dir, nil).Get(GetParams{Key: "tokens"})
	if err == nil {
		t.Fatal("expected a delimiter render error")
	}
	if strings.Contains(err.Error(), "a,b") {
		t.Errorf("error leaked the list item value: %v", err)
	}
}
