package get

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/fixtures"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
)

// -------------------------------------------------------------------------------------
// resolveBasic loads the shared "basic" fixture and resolves the api-core
// project for the default (development) environment.
func resolveBasic(t *testing.T) *engine.Result {
	t.Helper()
	m, err := manifest.New(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	ec, err := config.Resolve(m, "api-core", engine.Settings{}, nil)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	env, err := engine.Resolve(ec)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	return env
}

// -------------------------------------------------------------------------------------
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

// -------------------------------------------------------------------------------------
// TestRunActionMissing verifies an unknown key is an error.
func TestRunActionMissing(t *testing.T) {
	t.Parallel()

	env := resolveBasic(t)
	_, err := runAction(env, actionParams{Project: "api-core", Key: "nope"})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}
