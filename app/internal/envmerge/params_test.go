package envmerge

import "testing"

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

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
