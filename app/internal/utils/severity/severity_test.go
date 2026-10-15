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

func TestLevelRank(t *testing.T) {
	t.Parallel()

	if severity.None.Rank() >= severity.OK.Rank() {
		t.Errorf("Rank None (%d) should be < OK (%d)", severity.None.Rank(), severity.OK.Rank())
	}
	if severity.OK.Rank() >= severity.Warn.Rank() {
		t.Errorf("Rank OK (%d) should be < Warn (%d)", severity.OK.Rank(), severity.Warn.Rank())
	}
	if severity.Warn.Rank() >= severity.Error.Rank() {
		t.Errorf("Rank Warn (%d) should be < Error (%d)", severity.Warn.Rank(), severity.Error.Rank())
	}
}
