package set

import (
	"bytes"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestRenderReportsStorePath verifies safe metadata uses a dedicated path line.
func TestRenderReportsStorePath(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := render(&renderParams{
		Writer: &output,
		Result: actionResult{
			Group:     "production",
			Key:       "database_password",
			StorePath: "/workspace/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Stored secret \"database_password\" in group \"production\" at:\n" +
		"/workspace/secrets.yaml\n"
	if output.String() != want {
		t.Errorf("rendered output = %q, want %q", output.String(), want)
	}
}

// -------------------------------------------------------------------------------------

// TestRenderQuotesStorePathWithSpaces verifies paths with spaces remain
// unambiguous on their dedicated output line.
func TestRenderQuotesStorePathWithSpaces(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := render(&renderParams{
		Writer: &output,
		Result: actionResult{
			Group:     "production",
			Key:       "db_pass4",
			StorePath: "/workspace/my secrets/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Stored secret \"db_pass4\" in group \"production\" at:\n" +
		"\"/workspace/my secrets/secrets.yaml\"\n"
	if output.String() != want {
		t.Errorf("rendered output = %q, want %q", output.String(), want)
	}
}
