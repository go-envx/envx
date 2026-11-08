package hostenv_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

func TestCapture(t *testing.T) {
	t.Setenv("LIB_HOSTENV_PRESENT", "present_value")
	t.Setenv("LIB_HOSTENV_EMPTY", "")

	snapshot := hostenv.Capture()

	if got, ok := snapshot.Get("LIB_HOSTENV_PRESENT"); !ok || got != "present_value" {
		t.Errorf("Get(present) = (%q, %v), want (%q, true)", got, ok, "present_value")
	}
	if got, ok := snapshot.Get("LIB_HOSTENV_EMPTY"); !ok || got != "" {
		t.Errorf("Get(empty) = (%q, %v), want (%q, true)", got, ok, "")
	}
	if got, ok := snapshot.Get("LIB_HOSTENV_DEFINITELY_ABSENT"); ok || got != "" {
		t.Errorf("Get(absent) = (%q, %v), want (%q, false)", got, ok, "")
	}

	// Mutating the OS environment after Capture must not affect the frozen snapshot.
	t.Setenv("LIB_HOSTENV_PRESENT", "mutated_after_capture")
	if got, ok := snapshot.Get("LIB_HOSTENV_PRESENT"); !ok || got != "present_value" {
		t.Errorf(
			"snapshot was modified after OS environment change: got %q, want %q",
			got,
			"present_value",
		)
	}
}

func TestFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		environ  []string
		wantVars map[string]string
		absent   []string
	}{
		{
			name:     "nil slice",
			environ:  nil,
			wantVars: map[string]string{},
			absent:   []string{"FOO", "BAR"},
		},
		{
			name:     "empty slice",
			environ:  []string{},
			wantVars: map[string]string{},
			absent:   []string{"FOO"},
		},
		{
			name: "basic variables",
			environ: []string{
				"VAR1=value1",
				"VAR2=value2",
			},
			wantVars: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			absent: []string{"VAR3"},
		},
		{
			name: "empty values preserved",
			environ: []string{
				"EMPTY=",
				"NOT_EMPTY=x",
			},
			wantVars: map[string]string{
				"EMPTY":     "",
				"NOT_EMPTY": "x",
			},
			absent: []string{"OTHER"},
		},
		{
			name: "values containing multiple equals signs",
			environ: []string{
				"URL=https://example.com?a=1&b=2",
				"EQUALS=====",
			},
			wantVars: map[string]string{
				"URL":    "https://example.com?a=1&b=2",
				"EQUALS": "====",
			},
		},
		{
			name: "entries without equals sign ignored",
			environ: []string{
				"VALID=ok",
				"NO_EQUALS",
				"TRAILING_VALID=yes",
			},
			wantVars: map[string]string{
				"VALID":          "ok",
				"TRAILING_VALID": "yes",
			},
			absent: []string{"NO_EQUALS"},
		},
		{
			name: "duplicate keys use last supplied value",
			environ: []string{
				"KEY=first",
				"OTHER=val",
				"KEY=second",
				"KEY=third",
			},
			wantVars: map[string]string{
				"KEY":   "third",
				"OTHER": "val",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			snapshot := hostenv.From(tc.environ)

			for k, wantVal := range tc.wantVars {
				gotVal, ok := snapshot.Get(k)
				if !ok {
					t.Errorf("Get(%q) reported not found, want value %q", k, wantVal)
				} else if gotVal != wantVal {
					t.Errorf("Get(%q) = %q, want %q", k, gotVal, wantVal)
				}
			}

			for _, absentKey := range tc.absent {
				if gotVal, ok := snapshot.Get(absentKey); ok {
					t.Errorf("Get(%q) = (%q, true), want false", absentKey, gotVal)
				}
			}
		})
	}
}

func TestFrom_InputSliceIsolation(t *testing.T) {
	t.Parallel()

	input := []string{"KEY=original", "ANOTHER=1"}
	snapshot := hostenv.From(input)

	// Mutate input slice after calling From
	input[0] = "KEY=mutated"

	if got, ok := snapshot.Get("KEY"); !ok || got != "original" {
		t.Errorf(
			"snapshot was affected by mutating input slice: got %q, want %q",
			got,
			"original",
		)
	}
}

func TestZeroSnapshot(t *testing.T) {
	t.Parallel()

	var zero hostenv.Snapshot

	if got, ok := zero.Get("ANY_KEY"); ok || got != "" {
		t.Errorf("zero.Get() = (%q, %v), want (%q, false)", got, ok, "")
	}

	all := zero.All()
	if all == nil {
		t.Fatal("zero.All() returned nil, want non-nil empty map")
	}
	if len(all) != 0 {
		t.Errorf("len(zero.All()) = %d, want 0", len(all))
	}

	// Mutating map returned from zero.All() must not panic or affect subsequent calls
	all["TEST"] = "value"
	if len(zero.All()) != 0 {
		t.Errorf("mutating zero.All() leaked into zero snapshot")
	}
}

func TestAll_Isolation(t *testing.T) {
	t.Parallel()

	snapshot := hostenv.From([]string{"A=1", "B=2"})
	all := snapshot.All()

	if len(all) != 2 || all["A"] != "1" || all["B"] != "2" {
		t.Fatalf("All() = %+v, want map[A:1 B:2]", all)
	}

	// Mutate returned map
	all["A"] = "mutated"
	all["C"] = "new"
	delete(all, "B")

	// Snapshot must remain intact
	if got, ok := snapshot.Get("A"); !ok || got != "1" {
		t.Errorf("Get(A) after mutating All() = (%q, %v), want (1, true)", got, ok)
	}
	if got, ok := snapshot.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) after mutating All() = (%q, %v), want (2, true)", got, ok)
	}
	if _, ok := snapshot.Get("C"); ok {
		t.Error("Get(C) after mutating All() reported present")
	}

	secondAll := snapshot.All()
	if len(secondAll) != 2 || secondAll["A"] != "1" || secondAll["B"] != "2" {
		t.Errorf("subsequent All() was affected by mutation: %+v", secondAll)
	}
}
