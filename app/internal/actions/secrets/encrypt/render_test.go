package encrypt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/secrets"
)

// TestRenderReportsCountByDefault verifies the default summary reports the count
// and store location without listing each identity.
func TestRenderReportsCountByDefault(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := render(&renderParams{
		Writer: &out,
		Result: actionResult{
			Changed: []secrets.SecretReference{
				{Group: "production", Key: "api_key"},
				{Group: "shared", Key: "service_token"},
			},
			StorePath: "/workspace/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Encrypted 2 secret(s)") ||
		!strings.Contains(got, "/workspace/secrets.yaml") {
		t.Errorf("render() = %q, want the count and store location", got)
	}
	if strings.Contains(got, "production/api_key") {
		t.Errorf("render() = %q, want no per-identity list by default", got)
	}
}

// TestRenderListsIdentitiesWhenVerbose verifies verbose output lists each changed
// identity without any secret value.
func TestRenderListsIdentitiesWhenVerbose(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := render(&renderParams{
		Writer:  &out,
		Verbose: true,
		Result: actionResult{
			Changed: []secrets.SecretReference{
				{Group: "production", Key: "api_key"},
				{Group: "shared", Key: "service_token"},
			},
			StorePath: "/workspace/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"production/api_key", "shared/service_token", "/workspace/secrets.yaml",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("render() = %q, want it to contain %q", got, want)
		}
	}
}

// TestRenderReportsNothingChanged verifies an empty result reports plainly
// instead of an empty list.
func TestRenderReportsNothingChanged(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := render(&renderParams{Writer: &out, Result: actionResult{}}); err != nil {
		t.Fatalf("render(): %v", err)
	}
	if !strings.Contains(out.String(), "No plaintext values to encrypt.") {
		t.Errorf("render() = %q, want the nothing-to-do message", out.String())
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
