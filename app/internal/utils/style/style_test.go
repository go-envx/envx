package style

import (
	"strings"
	"testing"
)

// TestStylerDisabled verifies a disabled Styler returns text unchanged for every
// styling method.
func TestStylerDisabled(t *testing.T) {
	t.Parallel()

	s := New(false)
	const text = "value"
	styles := map[string]string{
		"Bold":   s.Bold(text),
		"Red":    s.Red(text),
		"Green":  s.Green(text),
		"Yellow": s.Yellow(text),
		"Muted":  s.Muted(text),
	}
	for name, got := range styles {
		if got != text {
			t.Errorf("%s() = %q, want %q", name, got, text)
		}
	}
	if s.Enabled() {
		t.Error("Enabled() = true, want false")
	}
}

// TestStylerEnabled verifies an enabled Styler wraps text in the expected SGR
// code and a reset.
func TestStylerEnabled(t *testing.T) {
	t.Parallel()

	s := New(true)
	if !s.Enabled() {
		t.Fatal("Enabled() = false, want true")
	}

	tests := []struct {
		name string
		got  string
		code string
	}{
		{"Bold", s.Bold("x"), "1"},
		{"Red", s.Red("x"), "31"},
		{"Green", s.Green("x"), "32"},
		{"Yellow", s.Yellow("x"), "33"},
		{"Cyan", s.Cyan("x"), "36"},
		{"Muted", s.Muted("x"), "90"},
	}
	for _, tt := range tests {
		want := "\033[" + tt.code + "mx\033[0m"
		if tt.got != want {
			t.Errorf("%s() = %q, want %q", tt.name, tt.got, want)
		}
	}
}

// TestStylerSeverity verifies severity levels map to the expected color and that
// SeverityNone leaves text unstyled even when enabled.
func TestStylerSeverity(t *testing.T) {
	t.Parallel()

	s := New(true)
	tests := []struct {
		name string
		sev  Severity
		want string
	}{
		{"ok", SeverityOK, s.Cyan("v")},
		{"warning", SeverityWarning, s.Yellow("v")},
		{"error", SeverityError, s.Red("v")},
		{"none", SeverityNone, "v"},
	}
	for _, tt := range tests {
		if got := s.Severity(tt.sev, "v"); got != tt.want {
			t.Errorf("Severity(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestStylerSeverityDisabled verifies severity coloring is suppressed when the
// Styler is disabled.
func TestStylerSeverityDisabled(t *testing.T) {
	t.Parallel()

	s := New(false)
	if got := s.Severity(SeverityError, "v"); got != "v" {
		t.Errorf("Severity(error) = %q, want %q", got, "v")
	}
	if strings.Contains(s.Severity(SeverityOK, "v"), "\033") {
		t.Error("disabled Severity emitted an escape code")
	}
}

// TestStylerColor verifies each named color maps to the expected style and that
// ColorNone leaves text unstyled even when enabled.
func TestStylerColor(t *testing.T) {
	t.Parallel()

	s := New(true)
	tests := []struct {
		name  string
		color Color
		want  string
	}{
		{"red", ColorRed, s.Red("v")},
		{"green", ColorGreen, s.Green("v")},
		{"yellow", ColorYellow, s.Yellow("v")},
		{"cyan", ColorCyan, s.Cyan("v")},
		{"muted", ColorMuted, s.Muted("v")},
		{"none", ColorNone, "v"},
	}
	for _, tt := range tests {
		if got := s.Color(tt.color, "v"); got != tt.want {
			t.Errorf("Color(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestStylerColorDisabled verifies named coloring is suppressed when the Styler
// is disabled.
func TestStylerColorDisabled(t *testing.T) {
	t.Parallel()

	s := New(false)
	if got := s.Color(ColorGreen, "v"); got != "v" {
		t.Errorf("Color(green) = %q, want %q", got, "v")
	}
}
