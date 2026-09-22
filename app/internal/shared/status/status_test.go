package status

import (
	"errors"
	"testing"
)

// TestDefaultSeverity verifies each reportable code carries the expected default
// and that a non-reportable code (OK or unknown) reports no finding.
func TestDefaultSeverity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		code    string
		wantSev Severity
		wantOK  bool
	}{
		{
			name: "encryption defaults to error",
			code: SecretIsNotEncrypted, wantSev: Error, wantOK: true,
		},
		{
			name: "orphan defaults to error",
			code: SecretIsNotReferenced, wantSev: Error, wantOK: true,
		},
		{
			name: "missing key defaults to warn",
			code: PrivateKeyIsUnavailable, wantSev: Warn, wantOK: true,
		},
		{
			name: "base declaration is a known check but defaults to off",
			code: PropertyNotDeclaredInBase, wantSev: Off, wantOK: true,
		},
		{name: "OK is not a finding", code: OK, wantOK: false},
		{name: "unknown is not a finding", code: "NOPE", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := DefaultSeverity(tc.code)
			if ok != tc.wantOK {
				t.Fatalf("DefaultSeverity(%q) ok = %v, want %v", tc.code, ok, tc.wantOK)
			}
			if ok && got != tc.wantSev {
				t.Errorf("DefaultSeverity(%q) = %q, want %q", tc.code, got, tc.wantSev)
			}
		})
	}
}

// TestParseSeverity verifies the three accepted levels round-trip and anything
// else is rejected with an InvalidSeverityError naming the offending value.
func TestParseSeverity(t *testing.T) {
	t.Parallel()

	for _, want := range []Severity{Off, Warn, Error} {
		got, err := ParseSeverity(string(want))
		if err != nil {
			t.Errorf("ParseSeverity(%q): %v", want, err)
		}
		if got != want {
			t.Errorf("ParseSeverity(%q) = %q, want %q", want, got, want)
		}
	}

	_, err := ParseSeverity("warning")
	var invalid *InvalidSeverityError
	if !errors.As(err, &invalid) {
		t.Fatalf("ParseSeverity(\"warning\") error = %v, want *InvalidSeverityError", err)
	}
	if invalid.Value != "warning" {
		t.Errorf("InvalidSeverityError.Value = %q, want %q", invalid.Value, "warning")
	}
}
