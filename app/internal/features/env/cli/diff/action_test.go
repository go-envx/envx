package diff

import (
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// executeBasic runs the diff action against the shared "basic" fixture for the
// api-core project between two environments.
func executeBasic(t *testing.T, envA, envB string) (actionResult, error) {
	t.Helper()
	path := fixtures.Manifest("basic")
	return execute(
		actionParams{Project: "api-core", EnvA: envA, EnvB: envB},
		&config.Input{ConfigPath: &path},
	)
}

// findChange returns the change with the given key and whether it was present.
func findChange(changes []actionResultChange, key string) (actionResultChange, bool) {
	for _, c := range changes {
		if c.Key == key {
			return c, true
		}
	}
	return actionResultChange{}, false
}

// TestExecuteMapsChange verifies the action wires config resolution through the
// manager and maps a DiffResult change into the env-a/env-b presentation shape.
func TestExecuteMapsChange(t *testing.T) {
	t.Parallel()

	res, err := executeBasic(t, "development", "production")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	c, ok := findChange(res.Changed, "HOST")
	if !ok {
		t.Fatalf("HOST missing from changed set: %+v", res.Changed)
	}
	if c.EnvA != "dev-db.local" || c.EnvB != "prod-db.internal" {
		t.Errorf(
			"HOST change = %q -> %q, want dev-db.local -> prod-db.internal",
			c.EnvA, c.EnvB,
		)
	}
}

// TestExecuteIdenticalEnvironments verifies diffing an environment against itself
// returns an empty result through the action.
func TestExecuteIdenticalEnvironments(t *testing.T) {
	t.Parallel()

	res, err := executeBasic(t, "development", "development")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Added) != 0 || len(res.Removed) != 0 || len(res.Changed) != 0 {
		t.Errorf("expected empty diff, got %+v", res)
	}
}

// TestExecuteUndeclaredEnvironment verifies an undeclared environment surfaces as
// an error from the action.
func TestExecuteUndeclaredEnvironment(t *testing.T) {
	t.Parallel()

	if _, err := executeBasic(t, "development", "ghost"); err == nil {
		t.Fatal("expected error for undeclared environment")
	}
}
