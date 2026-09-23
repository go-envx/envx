package validate

import (
	"testing"

	"github.com/go-envx/envx/app/internal/shared/status"
)

// TestFlagNameMatchesConfigKey verifies a check's selection flag name is the
// kebab-case twin of its lowercased config severity key, so the flag and the
// envx.yaml property stay aligned.
func TestFlagNameMatchesConfigKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		status.SecretIsNotEncrypted:      "secret-is-not-encrypted",
		status.SecretAlgorithmMismatch:   "secret-algorithm-mismatch",
		status.PublicKeyIsMissing:        "public-key-is-missing",
		status.PropertyNotDeclaredInBase: "property-not-declared-in-base",
	}
	for code, want := range cases {
		if got := FlagName(code); got != want {
			t.Errorf("FlagName(%q) = %q, want %q", code, got, want)
		}
	}
}

// TestRegistryCoversEveryReportableCode verifies every graded status code is in
// the registry exactly once, so no check is missing a group or a flag.
func TestRegistryCoversEveryReportableCode(t *testing.T) {
	t.Parallel()

	seen := map[string]int{}
	for _, check := range Checks() {
		seen[check.Code]++
		if _, ok := status.DefaultSeverity(check.Code); !ok {
			t.Errorf("check %q is not a reportable status code", check.Code)
		}
	}
	for _, code := range []string{
		status.SecretIsNotEncrypted, status.SecretAlgorithmMismatch,
		status.SecretIsNotReferenced, status.PublicKeyIsMissing,
		status.PrivateKeyIsInvalid, status.PrivateKeyIsUnavailable,
		status.SecretReferenceNotFound, status.InvalidSecretReference,
		status.SecretReferenceIsUnresolved, status.CircularVariableReference,
		status.UnresolvedVariableReference, status.PropertyNotDeclaredInBase,
	} {
		if seen[code] != 1 {
			t.Errorf("code %q appears %d times in the registry, want 1", code, seen[code])
		}
	}
}

// TestGroupOf verifies group lookup places store-owned and resolution-owned codes
// in their groups and rejects an unregistered code.
func TestGroupOf(t *testing.T) {
	t.Parallel()

	if g, ok := groupOf(status.SecretAlgorithmMismatch); !ok || g != GroupStore {
		t.Errorf("groupOf(algorithm) = %q,%v, want store,true", g, ok)
	}
	if g, ok := groupOf(status.PropertyNotDeclaredInBase); !ok || g != GroupResolution {
		t.Errorf("groupOf(base) = %q,%v, want resolution,true", g, ok)
	}
	if _, ok := groupOf("NOT_A_CODE"); ok {
		t.Error("groupOf(unknown) ok = true, want false")
	}
}

// TestNeedsMerge verifies the merge is needed only when a selected check reads
// it: no selection runs everything, a store-only selection skips the merge, and
// selecting the unused-secret store check still needs it for the reference set.
func TestNeedsMerge(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		selected map[string]bool
		want     bool
	}{
		{name: "no selection runs all", selected: nil, want: true},
		{
			name:     "store-only skips merge",
			selected: map[string]bool{status.SecretIsNotEncrypted: true},
			want:     false,
		},
		{
			name:     "unused-secret needs merge",
			selected: map[string]bool{status.SecretIsNotReferenced: true},
			want:     true,
		},
		{
			name:     "resolution check needs merge",
			selected: map[string]bool{status.PropertyNotDeclaredInBase: true},
			want:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := (Params{Selected: tc.selected}).needsMerge(); got != tc.want {
				t.Errorf("needsMerge() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRuns verifies selection semantics: with no selection every check runs; with
// a selection only the named checks run.
func TestRuns(t *testing.T) {
	t.Parallel()

	all := Params{}
	if !all.runs(status.PublicKeyIsMissing) {
		t.Error("no selection must run every check")
	}

	one := Params{Selected: map[string]bool{status.PublicKeyIsMissing: true}}
	if !one.runs(status.PublicKeyIsMissing) {
		t.Error("a selected check must run")
	}
	if one.runs(status.SecretIsNotEncrypted) {
		t.Error("an unselected check must not run")
	}
}
