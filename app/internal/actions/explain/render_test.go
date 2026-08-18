package explain

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/printer"
)

// plainPrinter builds a printer over the given sinks with color forced off so
// assertions can match exact, unstyled output.
func plainPrinter(out, errOut *bytes.Buffer) *printer.Printer {
	disabled := false
	return printer.New(printer.Options{Out: out, Err: errOut, Color: &disabled})
}

// sampleResult is a single ok config-value row used by the render tests.
func sampleResult() actionResult {
	return actionResult{
		Entries: []actionResultEntry{
			{
				Key:       "HOST",
				Literal:   "db.local",
				Source:    "env/postgres.development.yaml",
				SourceKey: "host",
				Shadowed:  []string{"env/postgres.yaml"},
				Resolution: envmerge.Resolution{
					Kind: envmerge.KindConfigValue, Severity: envmerge.SeverityOK, Code: "OK",
				},
			},
		},
	}
}

// TestRenderJSON verifies the JSON format emits a summary and a tagged entry
// array with the entry's status.
func TestRenderJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  sampleResult(),
		Format:  "json",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	want := `{
  "summary": {
    "severity": "ok",
    "errors": 0,
    "warnings": 0
  },
  "entries": [
    {
      "key": "HOST",
      "type": "config",
      "value": "db.local",
      "source": "env/postgres.development.yaml",
      "sourceKey": "host",
      "shadowed": [
        "env/postgres.yaml"
      ],
      "status": {
        "severity": "ok",
        "code": "OK"
      }
    }
  ]
}
`
	if buf.String() != want {
		t.Errorf("render json =\n%s\nwant\n%s", buf.String(), want)
	}
}

// TestRenderTable verifies the table format prints the column header and a row
// for each entry, without a RESOLVED column when reveal is off.
func TestRenderTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  sampleResult(),
		Format:  "table",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"KEY", "TYPE", "VALUE", "SOURCE", "STATUS",
		"HOST", "config", "db.local", "postgres.development.yaml", "OK",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "RESOLVED") {
		t.Errorf("table output should not include RESOLVED without reveal:\n%s", out)
	}
}

// TestRenderTableReveal verifies the RESOLVED column and its value appear only
// when reveal is requested.
func TestRenderTableReveal(t *testing.T) {
	t.Parallel()

	res := sampleResult()
	res.Entries[0].Resolution.Resolved = "db.local"
	res.Entries[0].Resolution.HasResolved = true

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  res,
		Format:  "table",
		Reveal:  true,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "RESOLVED") {
		t.Errorf("reveal table output missing RESOLVED column:\n%s", buf.String())
	}
}

// TestRenderTableBanner verifies an incomplete result leads with an ERROR banner
// on stderr while stdout carries only the table.
func TestRenderTableBanner(t *testing.T) {
	t.Parallel()

	res := actionResult{
		Entries: []actionResultEntry{
			{
				Key:     "TOKEN",
				Literal: "secret://dev/token",
				Source:  "env/app.yaml",
				Resolution: envmerge.Resolution{
					Kind:     envmerge.KindSecretReference,
					Severity: envmerge.SeverityError,
					Code:     "SECRET_NOT_FOUND",
				},
			},
		},
		Summary: envmerge.ExplanationSummary{Errors: 1},
	}

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  res,
		Format:  "table",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(errBuf.String(), "ERROR:") {
		t.Errorf("expected ERROR banner on stderr:\n%s", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "ERROR: 1 value failed to resolve") {
		t.Errorf("expected singular banner text:\n%q", errBuf.String())
	}
	if !strings.HasSuffix(errBuf.String(), "\n\n") {
		t.Errorf("expected blank line separating banner from table:\n%q", errBuf.String())
	}
	if strings.Contains(buf.String(), "ERROR:") {
		t.Errorf("banner should not appear on stdout:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "SECRET_NOT_FOUND") {
		t.Errorf("expected status code in table output:\n%s", buf.String())
	}
}

// TestRenderBannerSplitsAndPluralizes verifies a mixed outcome prints a separate
// error and warning line, each with correctly pluralized counts.
func TestRenderBannerSplitsAndPluralizes(t *testing.T) {
	t.Parallel()

	res := actionResult{
		Summary: envmerge.ExplanationSummary{Errors: 3, Warnings: 1},
	}

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  res,
		Format:  "table",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	got := errBuf.String()
	if !strings.Contains(got, "ERROR: 3 values failed to resolve\n") {
		t.Errorf("expected plural error line:\n%q", got)
	}
	if !strings.Contains(got, "WARNING: 1 value resolved with warnings\n") {
		t.Errorf("expected singular warning line:\n%q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("expected blank line after the banner:\n%q", got)
	}
}

// TestRenderJSONResolvedEmpty verifies a resolved empty value is emitted as an
// empty string, distinct from an unresolved value which is omitted.
func TestRenderJSONResolvedEmpty(t *testing.T) {
	t.Parallel()

	res := actionResult{
		Entries: []actionResultEntry{
			{
				Key:    "EMPTY",
				Source: "env/app.yaml",
				Resolution: envmerge.Resolution{
					Kind:        envmerge.KindConfigValue,
					Severity:    envmerge.SeverityOK,
					Code:        "OK",
					Resolved:    "",
					HasResolved: true,
				},
			},
			{
				Key:    "MISSING",
				Source: "env/app.yaml",
				Resolution: envmerge.Resolution{
					Kind:     envmerge.KindSecretReference,
					Severity: envmerge.SeverityError,
					Code:     "SECRET_NOT_FOUND",
				},
			},
		},
	}

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  res,
		Format:  "json",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var out struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}

	resolved, present := out.Entries[0]["resolved"]
	if !present || resolved != "" {
		t.Errorf("resolved empty value = %v (present %v), want present empty string",
			resolved, present)
	}
	if _, present := out.Entries[1]["resolved"]; present {
		t.Errorf("unresolved value should omit resolved, got %v", out.Entries[1])
	}
}

// TestRenderInvalidFormat verifies an unrecognized output format is rejected
// rather than silently falling back to the table.
func TestRenderInvalidFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  sampleResult(),
		Format:  "jsonn",
	})
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
}
