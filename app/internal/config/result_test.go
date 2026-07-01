package config

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
)

// -------------------------------------------------------------------------------------

// TestOverlayPath verifies the set action's overlay resolution from a project-less
// Resolve: the default environment feeds the overlay filename, and the error paths
// for an unknown include or an undeclared environment.
func TestOverlayPath(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")

	t.Run("default env and include", func(t *testing.T) {
		r, err := Resolve(&Input{ConfigPath: &path}, "")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		target, err := r.OverlayPath("env/postgres")
		if err != nil {
			t.Fatalf("OverlayPath: %v", err)
		}
		// The basic fixture declares [development, staging, production], so the
		// default resolves to the first declared environment.
		if filepath.Base(target) != "postgres.development.yaml" {
			t.Errorf("target = %q, want .../postgres.development.yaml", target)
		}
		if filepath.Base(filepath.Dir(target)) != "env" {
			t.Errorf("target dir = %q, want .../env", filepath.Dir(target))
		}
	})
	t.Run("unknown include errors", func(t *testing.T) {
		r, err := Resolve(&Input{ConfigPath: &path}, "")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if _, err := r.OverlayPath("env/ghost"); err == nil {
			t.Error("expected error for unknown include")
		}
	})
	t.Run("undeclared env errors", func(t *testing.T) {
		r, err := Resolve(&Input{ConfigPath: &path, Env: strPtr("nope")}, "")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if _, err := r.OverlayPath("env/postgres"); err == nil {
			t.Error("expected error for undeclared environment")
		}
	})
}
