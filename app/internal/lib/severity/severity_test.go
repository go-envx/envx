package severity_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/lib/severity"
)

func TestLevelConstants(t *testing.T) {
	t.Parallel()

	if severity.None != 0 {
		t.Errorf("None = %d, want 0", severity.None)
	}
	if severity.OK != 1 {
		t.Errorf("OK = %d, want 1", severity.OK)
	}
	if severity.Warn != 2 {
		t.Errorf("Warn = %d, want 2", severity.Warn)
	}
	if severity.Error != 3 {
		t.Errorf("Error = %d, want 3", severity.Error)
	}
}

func TestLevelString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		level severity.Level
		want  string
	}{
		{name: "none", level: severity.None, want: "none"},
		{name: "ok", level: severity.OK, want: "ok"},
		{name: "warn", level: severity.Warn, want: "warn"},
		{name: "error", level: severity.Error, want: "error"},
		{name: "unknown 4", level: severity.Level(4), want: "unknown"},
		{name: "unknown 99", level: severity.Level(99), want: "unknown"},
		{name: "unknown 255", level: severity.Level(255), want: "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.level.String(); got != tc.want {
				t.Errorf("Level(%d).String() = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestLevelDisplayString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		level severity.Level
		want  string
	}{
		{name: "none", level: severity.None, want: "none"},
		{name: "ok", level: severity.OK, want: "ok"},
		{name: "warn", level: severity.Warn, want: "warning"},
		{name: "error", level: severity.Error, want: "error"},
		{name: "unknown", level: severity.Level(100), want: "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.level.DisplayString(); got != tc.want {
				t.Errorf("Level(%d).DisplayString() = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestLevelRank(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		level severity.Level
		want  int
	}{
		{name: "none", level: severity.None, want: 0},
		{name: "ok", level: severity.OK, want: 1},
		{name: "warn", level: severity.Warn, want: 2},
		{name: "error", level: severity.Error, want: 3},
		{name: "custom level", level: severity.Level(42), want: 42},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.level.Rank(); got != tc.want {
				t.Errorf("Level(%d).Rank() = %d, want %d", tc.level, got, tc.want)
			}
		})
	}

	if severity.None.Rank() >= severity.OK.Rank() {
		t.Errorf(
			"Rank None (%d) should be < OK (%d)",
			severity.None.Rank(),
			severity.OK.Rank(),
		)
	}
	if severity.OK.Rank() >= severity.Warn.Rank() {
		t.Errorf(
			"Rank OK (%d) should be < Warn (%d)",
			severity.OK.Rank(),
			severity.Warn.Rank(),
		)
	}
	if severity.Warn.Rank() >= severity.Error.Rank() {
		t.Errorf(
			"Rank Warn (%d) should be < Error (%d)",
			severity.Warn.Rank(),
			severity.Error.Rank(),
		)
	}
}
