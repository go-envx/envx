package cli

import (
	"testing"
)

// TestGetCommandPrintsValue verifies the project, key, and --env reach the
// service and the value is printed with a trailing newline.
func TestGetCommandPrintsValue(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(
		t, NewGetCommand(newFakeFactory()), "app", "host", "--env", "production",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "prod-db.internal\n" {
		t.Errorf("stdout = %q, want prod-db.internal", stdout)
	}
}

// TestGetCommandSettings verifies setting flags reach the service.
func TestGetCommandSettings(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(
		t, NewGetCommand(newFakeFactory()), "app", "APP_PORT", "--prefix", "app",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "5432\n" {
		t.Errorf("stdout = %q, want 5432", stdout)
	}
}
