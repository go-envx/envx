package severity_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/utils/severity"
)

func TestLevelString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level severity.Level
		want  string
	}{
		{severity.None, "none"},
		{severity.OK, "ok"},
		{severity.Warn, "warn"},
		{severity.Error, "error"},
		{severity.Level(99), "unknown"},
	}

	for _, tc := range cases {
		if got := tc.level.String(); got != tc.want {
			t.Errorf("Level(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestLevelDisplayString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level severity.Level
		want  string
	}{
		{severity.None, "none"},
		{severity.OK, "ok"},
		{severity.Warn, "warning"},
		{severity.Error, "error"},
	}

	for _, tc := range cases {
		if got := tc.level.DisplayString(); got != tc.want {
			t.Errorf("Level(%d).DisplayString() = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestWorst(t *testing.T) {
	t.Parallel()

	if got := severity.Worst(severity.OK, severity.Warn); got != severity.Warn {
		t.Errorf("Worst(OK, Warn) = %v, want Warn", got)
	}
	if got := severity.Worst(severity.Error, severity.Warn); got != severity.Error {
		t.Errorf("Worst(Error, Warn) = %v, want Error", got)
	}
	if got := severity.Worst(severity.None, severity.OK); got != severity.OK {
		t.Errorf("Worst(None, OK) = %v, want OK", got)
	}
}
