package envmerge

import (
	"slices"
	"testing"
)

// TestNormalizeParamsDefaultsDelimiter verifies an empty delimiter falls back to
// the default comma while an explicit delimiter is left untouched.
func TestNormalizeParamsDefaultsDelimiter(t *testing.T) {
	t.Parallel()

	def, err := normalizeParams(Params{Environments: []string{"development"}})
	if err != nil {
		t.Fatalf("normalizeParams: %v", err)
	}
	if def.Settings.Delimiter != "," {
		t.Errorf("Delimiter = %q, want , (default)", def.Settings.Delimiter)
	}

	custom, err := normalizeParams(Params{
		Environments: []string{"development"},
		Settings:     Settings{Delimiter: ":"},
	})
	if err != nil {
		t.Fatalf("normalizeParams: %v", err)
	}
	if custom.Settings.Delimiter != ":" {
		t.Errorf("Delimiter = %q, want : (explicit)", custom.Settings.Delimiter)
	}
}

// TestNormalizeParamsCopiesSlices verifies the caller's Includes and Environments
// slices are copied, so later mutation cannot change manager behavior.
func TestNormalizeParamsCopiesSlices(t *testing.T) {
	t.Parallel()

	includes := []string{"a", "b"}
	environments := []string{"development", "production"}
	normalized, err := normalizeParams(Params{
		Includes:     includes,
		Environments: environments,
	})
	if err != nil {
		t.Fatalf("normalizeParams: %v", err)
	}

	includes[0] = "mutated"
	environments[0] = "mutated"
	if slices.Contains(normalized.Includes, "mutated") {
		t.Errorf("Includes shares the caller's array: %v", normalized.Includes)
	}
	if slices.Contains(normalized.Environments, "mutated") {
		t.Errorf("Environments shares the caller's array: %v", normalized.Environments)
	}
}

// TestNormalizeParamsDoesNotValidateEnvironment verifies construction defers
// environment validation: an undeclared default environment is accepted, since an
// explicit operation environment supersedes it.
func TestNormalizeParamsDoesNotValidateEnvironment(t *testing.T) {
	t.Parallel()

	if _, err := normalizeParams(Params{
		Environments:       []string{"development"},
		DefaultEnvironment: "undeclared",
	}); err != nil {
		t.Errorf("normalizeParams validated the environment: %v", err)
	}
}
