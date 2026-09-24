package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	engine "github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/shared/status"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// plainPrinter builds a printer over the given sinks with color forced off so
// assertions can match exact, unstyled output.
func plainPrinter(out, errOut *bytes.Buffer) *printer.Printer {
	disabled := false
	return printer.New(printer.Options{Out: out, Err: errOut, Color: &disabled})
}

// sampleReport is a mixed error/warning report used by the render tests.
func sampleReport() engine.Report {
	return engine.Report{
		Findings: []engine.Finding{
			{
				Severity: engine.SeverityError,
				Project:  "api", Environment: "production", Key: "PASSWORD",
				Code: status.SecretReferenceNotFound, Message: "no stored value for this reference",
			},
			{
				Severity: engine.SeverityWarning,
				Key:      "shared/unused", Code: status.SecretIsNotReferenced,
				Message: "stored value is never referenced by any environment",
			},
		},
		Errors: 1, Warnings: 1, Failed: true,
	}
}

// TestRenderTable verifies the table lists each finding with its scope and a
// severity banner precedes it.
func TestRenderTable(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	if err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Report:  sampleReport(),
	}); err != nil {
		t.Fatalf("render: %v", err)
	}

	body := out.String()
	for _, want := range []string{
		"SCOPE", "KEY", "STATUS", "MESSAGE",
		"api/production", "PASSWORD", "SECRET_REFERENCE_NOT_FOUND",
		"secrets", "shared/unused", "SECRET_IS_NOT_REFERENCED",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("table missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "KIND") {
		t.Errorf("table must not include the removed KIND column:\n%s", body)
	}

	banner := errOut.String()
	if !strings.Contains(banner, "1 error(s) found") {
		t.Errorf("banner missing error count:\n%s", banner)
	}
	if !strings.Contains(banner, "1 warning(s) found") {
		t.Errorf("banner missing warning count:\n%s", banner)
	}
}

// TestRenderTableSeparatesVerdict verifies a failing table is followed by a blank
// line — so the process boundary's "validation failed" verdict stands apart —
// while a passing report with findings is not. Both streams share one buffer so
// the interleaved write order (table then separator) is asserted faithfully.
func TestRenderTableSeparatesVerdict(t *testing.T) {
	t.Parallel()

	orphan := engine.Finding{
		Severity: engine.SeverityWarning,
		Key:      "shared/unused", Code: status.SecretIsNotReferenced,
		Message: "stored value is never referenced by any environment",
	}

	// A failing report ends with the table's final newline plus a separating
	// blank; a passing report ends with just the table's final newline.
	if got := renderCombined(t, engine.Report{
		Findings: []engine.Finding{orphan}, Errors: 1, Failed: true,
	}); !strings.HasSuffix(got, "\n\n") {
		t.Errorf("failing table must be followed by a blank line:\n%q", got)
	}
	if got := renderCombined(t, engine.Report{
		Findings: []engine.Finding{orphan}, Warnings: 1, Failed: false,
	}); strings.HasSuffix(got, "\n\n") {
		t.Errorf("passing table must not add a trailing blank line:\n%q", got)
	}
}

// renderCombined renders report with a single buffer behind both streams so the
// terminal-visible ordering of stdout and stderr writes is preserved, and returns
// what a viewer would see.
func renderCombined(t *testing.T, report engine.Report) string {
	t.Helper()
	var combined bytes.Buffer
	if err := render(&renderParams{
		Printer: plainPrinter(&combined, &combined),
		Report:  report,
	}); err != nil {
		t.Fatalf("render: %v", err)
	}
	return combined.String()
}

// TestRenderTableClean verifies a report with no findings prints a single
// confirmation line and no table.
func TestRenderTableClean(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	if err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Report:  engine.Report{},
	}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "no validation problems found" {
		t.Errorf("output = %q, want the clean confirmation line", got)
	}
	if strings.Contains(out.String(), "SCOPE") {
		t.Error("clean report must not print a table header")
	}
}

// TestRenderJSON verifies the JSON envelope carries the summary and a tagged
// findings array, omitting empty scope fields for a store-level finding.
func TestRenderJSON(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	if err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Report:  sampleReport(),
		Format:  "json",
	}); err != nil {
		t.Fatalf("render: %v", err)
	}

	var decoded jsonReport
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out.String())
	}
	summary := decoded.Summary
	if !summary.Failed || summary.Errors != 1 || summary.Warnings != 1 {
		t.Errorf("summary = %+v, want failed 1 error 1 warning", summary)
	}
	if len(decoded.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(decoded.Findings))
	}

	// The store-level orphan finding omits project and environment.
	if strings.Contains(out.String(), `"project": ""`) {
		t.Errorf("empty project should be omitted:\n%s", out.String())
	}
}

// TestRenderInvalidFormat verifies an unrecognized format is rejected loudly.
func TestRenderInvalidFormat(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Report:  sampleReport(),
		Format:  "jsonn",
	})
	if err == nil {
		t.Fatal("expected an error for an invalid format")
	}
}
