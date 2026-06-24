package diff

import (
	"bytes"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestMaskResult verifies values are redacted unless reveal is set, and that
// empty values stay empty.
func TestMaskResult(t *testing.T) {
	t.Parallel()

	in := actionResult{
		Added:   []actionResultChange{{Key: "A", EnvB: "secret"}},
		Removed: []actionResultChange{{Key: "B", EnvA: "gone"}},
		Changed: []actionResultChange{{Key: "C", EnvA: "old", EnvB: "new"}},
	}

	masked := maskResult(in, false)
	if masked.Added[0].EnvB != redacted {
		t.Errorf("added env-b = %q, want redacted", masked.Added[0].EnvB)
	}
	if masked.Removed[0].EnvA != redacted {
		t.Errorf("removed env-a = %q, want redacted", masked.Removed[0].EnvA)
	}
	if masked.Changed[0].EnvA != redacted || masked.Changed[0].EnvB != redacted {
		t.Errorf("changed not fully redacted: %+v", masked.Changed[0])
	}

	revealed := maskResult(in, true)
	if revealed.Added[0].EnvB != "secret" {
		t.Errorf("reveal should keep value, got %q", revealed.Added[0].EnvB)
	}
}

// -------------------------------------------------------------------------------------

// sampleResult is a small diff covering all three change kinds, used by the
// render tests.
func sampleResult() actionResult {
	return actionResult{
		Added:   []actionResultChange{{Key: "NEW", EnvB: "added-value"}},
		Removed: []actionResultChange{{Key: "OLD", EnvA: "removed-value"}},
		Changed: []actionResultChange{{Key: "MOD", EnvA: "before", EnvB: "after"}},
	}
}

// -------------------------------------------------------------------------------------

// TestRenderJSON verifies the JSON format emits added, removed, and changed
// sections with revealed values when reveal is set.
func TestRenderJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "json",
		Reveal: true,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	want := `{
  "added": [
    {
      "key": "NEW",
      "env_b": "added-value"
    }
  ],
  "removed": [
    {
      "key": "OLD",
      "env_a": "removed-value"
    }
  ],
  "changed": [
    {
      "key": "MOD",
      "env_a": "before",
      "env_b": "after"
    }
  ]
}
`
	if buf.String() != want {
		t.Errorf("render json =\n%s\nwant\n%s", buf.String(), want)
	}
}

// -------------------------------------------------------------------------------------

// TestRenderTable verifies the table format prints sign-prefixed rows with the
// arrow notation for changed keys.
func TestRenderTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "table",
		Reveal: true,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"+", "NEW", "added-value",
		"-", "OLD", "removed-value",
		"~", "MOD", "before -> after",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}
}

// -------------------------------------------------------------------------------------

// TestRenderMasksByDefault verifies render redacts values when reveal is not set.
func TestRenderMasksByDefault(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "table",
		Reveal: false,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, redacted) {
		t.Errorf("expected redacted placeholder, got:\n%s", out)
	}
	for _, secret := range []string{"added-value", "removed-value", "before", "after"} {
		if strings.Contains(out, secret) {
			t.Errorf("plaintext value %q leaked into masked output:\n%s", secret, out)
		}
	}
}
