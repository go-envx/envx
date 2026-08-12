package delete

import (
	"bytes"
	"strings"
	"testing"
)

// TestRenderReportsRemovedIdentity verifies the summary names the removed secret
// and its store location without any secret value.
func TestRenderReportsRemovedIdentity(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := render(&renderParams{
		Writer: &out,
		Result: actionResult{
			Group:     "production",
			Key:       "database_password",
			StorePath: "/workspace/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"database_password"`) ||
		!strings.Contains(got, `"production"`) {
		t.Errorf("render() = %q, want the removed identity", got)
	}
	if !strings.Contains(got, "/workspace/secrets.yaml") {
		t.Errorf("render() = %q, want the store location", got)
	}
}

// TestRenderStorePathQuotesSpaces verifies a path with a space is quoted so its
// boundary is unambiguous.
func TestRenderStorePathQuotesSpaces(t *testing.T) {
	t.Parallel()

	if got := renderStorePath("/no/space"); got != "/no/space" {
		t.Errorf("renderStorePath() = %q, want the raw path", got)
	}
	const spaced = "/with space/secrets.yaml"
	if got := renderStorePath(spaced); got != `"`+spaced+`"` {
		t.Errorf("renderStorePath() = %q, want a quoted path", got)
	}
}
