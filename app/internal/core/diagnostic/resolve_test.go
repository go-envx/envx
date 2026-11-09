package diagnostic_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	got, err := diagnostic.Resolve(map[string]string{
		"secret_is_not_referenced":      "off",
		"PRIVATE_KEY_IS_UNAVAILABLE":    "error",
		"property_not_declared_in_base": "warn",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := map[diagnostic.Code]severity.Level{
		diagnostic.SecretIsNotReferenced:     severity.None,
		diagnostic.PrivateKeyIsUnavailable:   severity.Error,
		diagnostic.PropertyNotDeclaredInBase: severity.Warn,
	}
	if len(got) != len(want) {
		t.Fatalf("Resolve() = %v, want %v", got, want)
	}
	for code, level := range want {
		if got[code] != level {
			t.Errorf("Resolve()[%s] = %v, want %v", code, got[code], level)
		}
	}
}

func TestResolve_Empty(t *testing.T) {
	t.Parallel()

	for _, in := range []map[string]string{nil, {}} {
		got, err := diagnostic.Resolve(in)
		if err != nil || got != nil {
			t.Errorf("Resolve(%v) = %v, %v; want nil, nil", in, got, err)
		}
	}
}

func TestResolve_Errors(t *testing.T) {
	t.Parallel()

	_, err := diagnostic.Resolve(map[string]string{"not_a_check": "warn"})
	var unknown *diagnostic.UnknownCodeError
	if !errors.As(err, &unknown) || unknown.Key != "not_a_check" {
		t.Errorf("unknown key error = %v, want UnknownCodeError", err)
	}

	_, err = diagnostic.Resolve(map[string]string{"ok": "warn"})
	if !errors.As(err, &unknown) {
		t.Errorf("OK key error = %v, want UnknownCodeError", err)
	}

	for _, raw := range []string{"warning", "Error", "none", ""} {
		_, err = diagnostic.Resolve(map[string]string{"secret_is_not_encrypted": raw})
		var invalid *diagnostic.InvalidSeverityError
		if !errors.As(err, &invalid) || invalid.Value != raw {
			t.Errorf("Resolve(%q) error = %v, want InvalidSeverityError", raw, err)
		}
	}
}
