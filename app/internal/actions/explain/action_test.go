package explain

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// executeBasic runs the explain action over the shared "basic" fixture for the
// api-core project, returning the presentation result.
func executeBasic(t *testing.T, p actionParams) actionResult {
	t.Helper()
	path := fixtures.Manifest("basic")
	p.Project = "api-core"
	res, err := execute(p, &config.Input{ConfigPath: &path})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return res
}

// findEntry returns the entry with the given key from a result, and whether it
// was present.
func findEntry(res actionResult, key string) (actionResultEntry, bool) {
	for i := range res.Entries {
		if res.Entries[i].Key == key {
			return res.Entries[i], true
		}
	}
	return actionResultEntry{}, false
}

// TestExecuteAllKeys verifies an empty key explains every key, each entry
// carrying its literal value and a workspace-relative source path.
func TestExecuteAllKeys(t *testing.T) {
	t.Parallel()

	res := executeBasic(t, actionParams{})
	if len(res.Entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	host, ok := findEntry(res, "HOST")
	if !ok {
		t.Fatalf("HOST missing from entries: %+v", res.Entries)
	}
	if host.Literal != "dev-db.local" {
		t.Errorf("HOST value = %q, want dev-db.local", host.Literal)
	}
	if host.Source != filepath.Join("env", "postgres.development.yaml") {
		t.Errorf("HOST source = %q, want env/postgres.development.yaml", host.Source)
	}
	if host.Resolution.Kind != envmerge.KindConfigValue {
		t.Errorf("HOST kind = %q, want config", host.Resolution.Kind)
	}
}

// TestExecuteSpecificKey verifies a case-insensitive key explains just that key
// and reports its origin.
func TestExecuteSpecificKey(t *testing.T) {
	t.Parallel()

	res := executeBasic(t, actionParams{Key: "host"})
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

// TestExecuteAbsoluteSource verifies --absolute renders the winning source as an
// absolute path.
func TestExecuteAbsoluteSource(t *testing.T) {
	t.Parallel()

	res := executeBasic(t, actionParams{Absolute: true})
	host, ok := findEntry(res, "HOST")
	if !ok {
		t.Fatalf("HOST missing from entries: %+v", res.Entries)
	}
	if !filepath.IsAbs(host.Source) {
		t.Errorf("HOST source = %q, want an absolute path", host.Source)
	}
}

// TestExecuteMissingKey verifies an unknown key is an error.
func TestExecuteMissingKey(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	_, err := execute(
		actionParams{Project: "api-core", Key: "nope"},
		&config.Input{ConfigPath: &path},
	)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

// TestSourcePathRelative verifies a source inside the workspace renders relative
// to the workspace root.
func TestSourcePathRelative(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join("home", "user", "project")
	source := filepath.Join(workspace, "env", "postgres.yaml")
	got := sourcePath(workspace, source, false)
	if got != filepath.Join("env", "postgres.yaml") {
		t.Errorf("sourcePath = %q, want env/postgres.yaml", got)
	}
}

// TestSourcePathOutsideWorkspace verifies a source outside the workspace falls
// back to its absolute path rather than a "../" escaped relative path.
func TestSourcePathOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join("home", "user", "project")
	source := filepath.Join("home", "user", "shared", "secrets.yaml")
	got := sourcePath(workspace, source, false)
	if got != source {
		t.Errorf("sourcePath = %q, want the absolute source %q", got, source)
	}
}

// TestSourcePathAbsolute verifies the absolute flag returns the source unchanged.
func TestSourcePathAbsolute(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join("home", "user", "project")
	source := filepath.Join(workspace, "env", "postgres.yaml")
	if got := sourcePath(workspace, source, true); got != source {
		t.Errorf("sourcePath absolute = %q, want %q", got, source)
	}
}
