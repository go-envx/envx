package explain

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// -------------------------------------------------------------------------------------

// resolveBasic loads the shared "basic" fixture and resolves the api-core
// project for the default environment (the first declared).
func resolveBasic(t *testing.T) *envmerge.Result {
	t.Helper()
	path := fixtures.Manifest("basic")
	r, err := config.Resolve(&config.Input{ConfigPath: &path}, "api-core")
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	env, err := envmerge.Build(r.Envmerge)
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return env
}

// -------------------------------------------------------------------------------------

// findEntry returns the entry with the given key from a result, and whether it
// was present.
func findEntry(res actionResult, key string) (actionResultEntry, bool) {
	for _, e := range res.Entries {
		if e.Key == key {
			return e, true
		}
	}
	return actionResultEntry{}, false
}

// -------------------------------------------------------------------------------------

// TestRunActionAllKeys verifies an empty key explains every resolved key, with
// each entry carrying the file that provided its value.
func TestRunActionAllKeys(t *testing.T) {
	t.Parallel()

	env := resolveBasic(t)
	res, err := runAction(env, actionParams{Project: "api-core"})
	if err != nil {
		t.Fatalf("runAction: %v", err)
	}
	if len(res.Entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	host, ok := findEntry(res, "HOST")
	if !ok {
		t.Fatalf("HOST missing from entries: %+v", res.Entries)
	}
	if host.Value != "dev-db.local" {
		t.Errorf("HOST value = %q, want dev-db.local", host.Value)
	}
	if filepath.Base(host.Source) != "postgres.development.yaml" {
		t.Errorf("HOST source = %q, want postgres.development.yaml", host.Source)
	}
}

// -------------------------------------------------------------------------------------

// TestRunActionSpecificKey verifies a case-insensitive key explains just that
// key and reports its origin.
func TestRunActionSpecificKey(t *testing.T) {
	t.Parallel()

	env := resolveBasic(t)
	res, err := runAction(env, actionParams{Project: "api-core", Key: "host"})
	if err != nil {
		t.Fatalf("runAction: %v", err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("expected exactly one entry, got %d", len(res.Entries))
	}
	if res.Entries[0].Key != "HOST" {
		t.Errorf("Key = %q, want HOST", res.Entries[0].Key)
	}
	if res.Entries[0].SourceKey != "host" {
		t.Errorf("SourceKey = %q, want host", res.Entries[0].SourceKey)
	}
}

// -------------------------------------------------------------------------------------

// TestRunActionMissingKey verifies an unknown key is an error.
func TestRunActionMissingKey(t *testing.T) {
	t.Parallel()

	env := resolveBasic(t)
	_, err := runAction(env, actionParams{Project: "api-core", Key: "nope"})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}
