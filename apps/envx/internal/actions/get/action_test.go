package get

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/fixtures"
)

// -------------------------------------------------------------------------------------
// noChange is a config.FlagSet stub reporting that no flag was set.
type noChange struct{}

// -------------------------------------------------------------------------------------
// Changed always reports false.
func (noChange) Changed(string) bool { return false }

// -------------------------------------------------------------------------------------
// resolveBasic loads the shared "basic" fixture and resolves the api-core
// environment for the development environment.
func resolveBasic(t *testing.T) *engine.Result {
	t.Helper()
	cfg, err := config.Load(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	env, err := engine.ResolveEnv(&engine.Request{
		Global:  config.Global{Config: cfg, Environment: "development"},
		Project: "api-core",
		Changed: noChange{},
	})
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
