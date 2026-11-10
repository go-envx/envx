package cli

import (
	"strings"
	"testing"
)

// TestSecretsDeleteCommand verifies the named secret is removed and reported.
func TestSecretsDeleteCommand(t *testing.T) {
	t.Parallel()

	factory := seededFactory(t)
	stdout, _, err := execute(
		t, factory, "", "secrets", "delete", "production", "api_key",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if factory.stored("production", "api_key") != "" {
		t.Error("api_key is still stored")
	}
	want := `Deleted secret "api_key" from group "production" in:` + "\nmem://secrets"
	if !strings.Contains(stdout, want) {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}
