package decrypt

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
	if !strings.Contains(got, "Decrypted 2 secret(s)") ||
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
	if !strings.Contains(out.String(), "No encrypted values to decrypt.") {
		t.Errorf("render() = %q, want the nothing-to-do message", out.String())
	}
}

// TestRenderWarnsOnUnavailableGroups verifies skipped groups are reported on
// stderr while the decrypted summary stays on stdout.
func TestRenderWarnsOnUnavailableGroups(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Writer:    &out,
		ErrWriter: &errOut,
		Verbose:   true,
		Result: actionResult{
			Changed:     []secrets.SecretReference{{Group: "dev", Key: "api_key"}},
			Unavailable: []string{"prd"},
			StorePath:   "/workspace/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	if !strings.Contains(out.String(), "dev/api_key") {
		t.Errorf("stdout = %q, want the decrypted summary", out.String())
	}
	warning := errOut.String()
	if !strings.Contains(warning, "warning:") || !strings.Contains(warning, `"prd"`) {
		t.Errorf("stderr = %q, want a warning naming the skipped group", warning)
	}
	if strings.Contains(warning, ansiYellow) {
		t.Errorf("stderr = %q, want no color when Color is false", warning)
	}
}

// TestRenderColorizesWarningOnTerminal verifies the warning label is colored when
// standard error is a terminal.
func TestRenderColorizesWarningOnTerminal(t *testing.T) {
	t.Parallel()

	var errOut bytes.Buffer
	err := render(&renderParams{
		Writer:    &bytes.Buffer{},
		ErrWriter: &errOut,
		Color:     true,
		Result:    actionResult{Unavailable: []string{"prd"}},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	if !strings.Contains(errOut.String(), ansiYellow) {
		t.Errorf("stderr = %q, want the warning colorized", errOut.String())
	}
}

// TestRenderSilentSummaryWhenAllSkipped verifies stdout stays empty when nothing
// could be decrypted but a group was skipped, since stderr explains it.
func TestRenderSilentSummaryWhenAllSkipped(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Writer:    &out,
		ErrWriter: &errOut,
		Result:    actionResult{Unavailable: []string{"prd"}},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty when everything was skipped", out.String())
	}
	if !strings.Contains(errOut.String(), `"prd"`) {
		t.Errorf("stderr = %q, want a warning", errOut.String())
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
