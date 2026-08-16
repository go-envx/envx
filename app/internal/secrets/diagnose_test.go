package secrets

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
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

// diagnoseRawResolver builds a revealing resolver over a store whose single
// production/database_password entry holds storedValue verbatim, so a test can
// diagnose non-envelope and algorithm-mismatched values.
func diagnoseRawResolver(t *testing.T, storedValue string) *Resolver {
	t.Helper()
	selected := newTestCipher(t)
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	storePath := writeStore(t,
		"public_keys:\n  production: "+pair.PublicKey+
			"\nsecrets:\n  production:\n    database_password: \""+storedValue+"\"\n",
	)
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                selected,
		PrivateKeyResolver:    fixedPrivateKeyResolver{value: pair.PrivateKey},
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	r, err := manager.Resolver(ResolverParams{Reveal: true})
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

// TestDiagnoseEscapedReference verifies a backslash-escaped reference is an ok
// config value carrying its unescaped literal, materialized only under reveal.
func TestDiagnoseEscapedReference(t *testing.T) {
	t.Parallel()

	const escaped = `\secret://production/database_password`

	masked := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := masked.Diagnose(escaped, "")
	if res.Kind != envmerge.KindConfigValue || res.Severity != envmerge.SeverityOK {
		t.Errorf("status = %s/%s, want config_value/ok", res.Kind, res.Severity)
	}
	if res.HasResolved {
		t.Error("masked escaped reference should not materialize plaintext")
	}

	revealed := diagnoseResolver(t, true, fixedPrivateKeyResolver{})
	got := revealed.Diagnose(escaped, "")
	if !got.HasResolved || got.Resolved != "secret://production/database_password" {
		t.Errorf("revealed escaped reference = %+v, want the unescaped literal", got)
	}
}

// TestDiagnoseInvalidReference verifies malformed reference grammar is an error
// classified as INVALID_REFERENCE.
func TestDiagnoseInvalidReference(t *testing.T) {
	t.Parallel()

	r := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := r.Diagnose("secret://database_password", "")
	if res.Kind != envmerge.KindSecretReference {
		t.Errorf("Kind = %q, want secret_reference", res.Kind)
	}
	if res.Severity != envmerge.SeverityError || res.Code != CodeInvalidReference {
		t.Errorf("status = %s/%s, want error/INVALID_REFERENCE", res.Severity, res.Code)
	}
}

// TestDiagnoseNotEncrypted verifies a stored value that is not an envelope is an
// error classified as NOT_ENCRYPTED.
func TestDiagnoseNotEncrypted(t *testing.T) {
	t.Parallel()

	r := diagnoseRawResolver(t, "plain-not-an-envelope")
	res := r.Diagnose("secret://production/database_password", "")
	if res.Severity != envmerge.SeverityError || res.Code != CodeNotEncrypted {
		t.Errorf("status = %s/%s, want error/NOT_ENCRYPTED", res.Severity, res.Code)
	}
}

// TestDiagnoseAlgorithmMismatch verifies an envelope tagged with a different
// algorithm than the configured cipher is an error classified as
// ALGORITHM_MISMATCH.
func TestDiagnoseAlgorithmMismatch(t *testing.T) {
	t.Parallel()

	mismatched, err := envelope.Encode(cipher.NaClBox, []byte("payload-bytes"))
	if err != nil {
		t.Fatalf("Encode(): %v", err)
	}

	r := diagnoseRawResolver(t, mismatched)
	res := r.Diagnose("secret://production/database_password", "")
	if res.Severity != envmerge.SeverityError || res.Code != CodeAlgorithmMismatch {
		t.Errorf("status = %s/%s, want error/ALGORITHM_MISMATCH", res.Severity, res.Code)
	}
}

// TestDiagnoseInvalidPrivateKey verifies a well-formed private key that does not
// decrypt the stored value is an error classified as INVALID_PRIVATE_KEY.
func TestDiagnoseInvalidPrivateKey(t *testing.T) {
	t.Parallel()

	// A second, independent keypair yields a well-formed key that cannot decrypt
	// a value encrypted for the store's own public key.
	wrong, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	manager := newGetManager(t, fixedPrivateKeyResolver{value: wrong.PrivateKey})
	if err := manager.Set("production", "database_password", func() (string, error) {
		return "database-password", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	r, err := manager.Resolver(ResolverParams{Reveal: true})
	if err != nil {
		t.Fatalf("Resolver(): %v", err)
	}

	res := r.Diagnose("secret://production/database_password", "")
	if res.Severity != envmerge.SeverityError || res.Code != CodeInvalidPrivateKey {
		t.Errorf("status = %s/%s, want error/INVALID_PRIVATE_KEY", res.Severity, res.Code)
	}
}
