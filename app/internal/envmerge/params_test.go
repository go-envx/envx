package envmerge

import "testing"

// TestNormalizeParamsDefaultsEnv verifies an empty target environment falls back
// to the first declared environment.
func TestNormalizeParamsDefaultsEnv(t *testing.T) {
	t.Parallel()

	p := &Params{Environments: []string{"development", "production"}}
	if err := normalizeParams(p); err != nil {
		t.Fatalf("normalizeParams: %v", err)
	}
	if p.Settings.Env != "development" {
		t.Errorf("Env = %q, want development", p.Settings.Env)
	}
}

// TestNormalizeParamsKeepsExplicitEnv verifies an explicitly set environment is
// left untouched.
func TestNormalizeParamsKeepsExplicitEnv(t *testing.T) {
	t.Parallel()

	p := &Params{
		Environments: []string{"development", "production"},
		Settings:     Settings{Env: "production"},
	}
	if err := normalizeParams(p); err != nil {
		t.Fatalf("normalizeParams: %v", err)
	}
	if p.Settings.Env != "production" {
		t.Errorf("Env = %q, want production", p.Settings.Env)
	}
}

// TestNormalizeParamsDefaultsDelimiter verifies an empty delimiter falls back to
// the default comma while an explicit delimiter is left untouched.
func TestNormalizeParamsDefaultsDelimiter(t *testing.T) {
	t.Parallel()

	def := &Params{Environments: []string{"development"}}
	if err := normalizeParams(def); err != nil {
		t.Fatalf("normalizeParams: %v", err)
	}
	if def.Settings.Delimiter != "," {
		t.Errorf("Delimiter = %q, want , (default)", def.Settings.Delimiter)
	}

	custom := &Params{
		Environments: []string{"development"},
		Settings:     Settings{Delimiter: ":"},
	}
	if err := normalizeParams(custom); err != nil {
		t.Fatalf("normalizeParams: %v", err)
	}
	if custom.Settings.Delimiter != ":" {
		t.Errorf("Delimiter = %q, want : (explicit)", custom.Settings.Delimiter)
	}
}

// TestNormalizeParamsUndeclaredEnv verifies an environment outside the declared
// set is rejected.
func TestNormalizeParamsUndeclaredEnv(t *testing.T) {
	t.Parallel()

	p := &Params{
		Environments: []string{"development"},
		Settings:     Settings{Env: "nope"},
	}
	if err := normalizeParams(p); err == nil {
		t.Error("expected error for undeclared environment")
	}
}
