package get

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// executeBasic runs the get action against the shared "basic" fixture for the
// api-core project and the given key.
func executeBasic(t *testing.T, key string) (actionResult, error) {
	t.Helper()
	path := fixtures.Manifest("basic")
	return execute(
		actionParams{Project: "api-core", Key: key},
		&config.Input{ConfigPath: &path},
	)
}

// TestExecuteFound verifies a case-insensitive hit returns the value and its
// source file.
func TestExecuteFound(t *testing.T) {
	t.Parallel()

	res, err := executeBasic(t, "host")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Value != "dev-db.local" {
		t.Errorf("Value = %q, want dev-db.local", res.Value)
	}
	if filepath.Base(res.Source) != "postgres.development.yaml" {
		t.Errorf("Source = %q, want postgres.development.yaml", res.Source)
	}
}

// TestExecuteMissing verifies an unknown key is an error.
func TestExecuteMissing(t *testing.T) {
	t.Parallel()

	if _, err := executeBasic(t, "nope"); err == nil {
		t.Fatal("expected error for missing key")
	}
}
