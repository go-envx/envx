package get

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// danglingResolver resolves a known set of references and fails all others,
// standing in for a secrets store that holds only some referenced entries.
type danglingResolver struct {
	// known maps a resolvable reference to its value.
	known map[string]string
}

// Resolve returns a known reference's value, passes plain values through, and
// fails an unknown reference as a dangling reference.
func (r danglingResolver) Resolve(value, _ string) (string, error) {
	if !strings.HasPrefix(value, "secret://") {
		return value, nil
	}
	if v, ok := r.known[value]; ok {
		return v, nil
	}
	return "", fmt.Errorf("secret %q not found", value)
}

// resolveBasic loads the shared "basic" fixture and resolves the api-core
// project for the default environment (the first declared).
func resolveBasic(t *testing.T) *envmerge.Result {
	t.Helper()
	path := fixtures.Manifest("basic")
	r, err := config.ResolveProject(&config.Input{ConfigPath: &path}, "api-core", false)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	env, err := envmerge.Build(r.Envmerge)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	return env
}

// TestRunActionFound verifies a case-insensitive hit returns the value and its
// source file.
func TestRunActionFound(t *testing.T) {
	t.Parallel()

	env := resolveBasic(t)
	res, err := runAction(env, actionParams{Project: "api-core", Key: "host"})
	if err != nil {
		t.Fatalf("runAction: %v", err)
	}
	if res.Value != "dev-db.local" {
		t.Errorf("Value = %q, want dev-db.local", res.Value)
	}
	if filepath.Base(res.Source) != "postgres.development.yaml" {
		t.Errorf("Source = %q, want postgres.development.yaml", res.Source)
	}
}

// TestRunActionMissing verifies an unknown key is an error.
func TestRunActionMissing(t *testing.T) {
	t.Parallel()

	env := resolveBasic(t)
	_, err := runAction(env, actionParams{Project: "api-core", Key: "nope"})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

// buildDanglingEnv resolves a namespace with one good reference and one dangling
// reference, so a test can check that get isolates a single key.
func buildDanglingEnv(t *testing.T) *envmerge.Result {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "app.yaml"),
		[]byte("good: secret://ok\nbad: secret://missing\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	env, err := envmerge.Build(envmerge.Params{
		Includes:     []string{filepath.Join(dir, "app")},
		Environments: []string{"development"},
		ValueResolver: danglingResolver{
			known: map[string]string{"secret://ok": "value"},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return env
}

// TestRunActionIgnoresUnrelatedDanglingRef verifies get returns a requested key's
// value even when a different key holds an unresolved reference.
func TestRunActionIgnoresUnrelatedDanglingRef(t *testing.T) {
	t.Parallel()

	env := buildDanglingEnv(t)
	res, err := runAction(env, actionParams{Project: "app", Key: "good"})
	if err != nil {
		t.Fatalf("runAction(good): %v", err)
	}
	if res.Value != "value" {
		t.Errorf("Value = %q, want value", res.Value)
	}
}

// TestRunActionReportsRequestedKeyDanglingRef verifies get surfaces the requested
// key's own resolution failure rather than reporting it as merely absent.
func TestRunActionReportsRequestedKeyDanglingRef(t *testing.T) {
	t.Parallel()

	env := buildDanglingEnv(t)
	_, err := runAction(env, actionParams{Project: "app", Key: "bad"})
	if err == nil {
		t.Fatal("runAction(bad) should surface the dangling reference error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want the resolver's dangling message", err)
	}
}
