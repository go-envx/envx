package secrets

import (
	"testing"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/privatekey"
)

// diagnoseResolver builds a resolver over a manager that has one stored secret,
// with the requested reveal policy.
func diagnoseResolver(
	t *testing.T, reveal bool, resolver privatekey.Resolver,
) *Resolver {
	t.Helper()
	manager := newGetManager(t, resolver)
	if err := manager.Set("production", "database_password", func() (string, error) {
		return "database-password", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	r, err := manager.Resolver(ResolverParams{Reveal: reveal})
	if err != nil {
		t.Fatalf("Resolver(): %v", err)
	}
	return r
}

// TestDiagnoseConfigValue verifies a plain value is classified as an ok config
// value, materialized only under reveal.
func TestDiagnoseConfigValue(t *testing.T) {
	t.Parallel()

	r := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := r.Diagnose("plain-value", "")
	if res.Kind != envmerge.KindConfigValue {
		t.Errorf("Kind = %q, want config_value", res.Kind)
	}
	if res.Severity != envmerge.SeverityOK || res.Code != CodeOK {
		t.Errorf("status = %s/%s, want ok/OK", res.Severity, res.Code)
	}
	if res.HasResolved {
		t.Error("masked config value should not materialize plaintext")
	}

	revealed := diagnoseResolver(t, true, fixedPrivateKeyResolver{})
	if got := revealed.Diagnose("plain-value", ""); !got.HasResolved ||
		got.Resolved != "plain-value" {
		t.Errorf("revealed config value = %+v, want resolved plain-value", got)
	}
}

// TestDiagnoseSecretReference verifies a resolvable reference reports an ok
// secret status, materialized only under reveal.
func TestDiagnoseSecretReference(t *testing.T) {
	t.Parallel()

	masked := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := masked.Diagnose("secret://production/database_password", "")
	if res.Kind != envmerge.KindSecretReference {
		t.Errorf("Kind = %q, want secret_reference", res.Kind)
	}
	if res.Severity != envmerge.SeverityOK || res.Code != CodeOK {
		t.Errorf("status = %s/%s, want ok/OK", res.Severity, res.Code)
	}
	if res.HasResolved {
		t.Error("masked secret should not materialize plaintext")
	}

	revealed := diagnoseResolver(t, true, fixedPrivateKeyResolver{})
	got := revealed.Diagnose("secret://production/database_password", "")
	if !got.HasResolved || got.Resolved != "database-password" {
		t.Errorf("revealed secret = %+v, want resolved plaintext", got)
	}
}

// TestDiagnoseDanglingReference verifies a reference with no stored value is an
// error classified as SECRET_NOT_FOUND.
func TestDiagnoseDanglingReference(t *testing.T) {
	t.Parallel()

	r := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := r.Diagnose("secret://production/missing", "")
	if res.Severity != envmerge.SeverityError || res.Code != CodeSecretNotFound {
		t.Errorf("status = %s/%s, want error/SECRET_NOT_FOUND", res.Severity, res.Code)
	}
}

// TestDiagnoseUnavailableKey verifies a reference whose group has no private key
// is a warning classified as PRIVATE_KEY_UNAVAILABLE, never an abort.
func TestDiagnoseUnavailableKey(t *testing.T) {
	t.Parallel()

	r := diagnoseResolver(t, false, newPrivateKeyTestResolver())
	res := r.Diagnose("secret://production/database_password", "")
	if res.Severity != envmerge.SeverityWarning ||
		res.Code != CodePrivateKeyUnavailable {
		t.Errorf(
			"status = %s/%s, want warning/PRIVATE_KEY_UNAVAILABLE",
			res.Severity, res.Code,
		)
	}
}
