package secrets

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/shared/status"
	"github.com/go-envx/envx/app/internal/utils/severity"
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
	res := r.Evaluate("plain-value", "")
	if res.Severity != severity.OK || res.Status != status.OK {
		t.Errorf("status = %s/%s, want ok/OK", res.Severity, res.Status)
	}
	if res.IsResolved {
		t.Error("masked config value should not materialize plaintext")
	}

	revealed := diagnoseResolver(t, true, fixedPrivateKeyResolver{})
	if got := revealed.Evaluate("plain-value", ""); !got.IsResolved ||
		got.Value != "plain-value" {
		t.Errorf("revealed config value = %+v, want resolved plain-value", got)
	}
}

// TestDiagnoseSecretReference verifies a resolvable reference reports an ok
// secret status, materialized only under reveal.
func TestDiagnoseSecretReference(t *testing.T) {
	t.Parallel()

	masked := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := masked.Evaluate("secret://production/database_password", "")
	if res.Severity != severity.OK || res.Status != status.OK {
		t.Errorf("status = %s/%s, want ok/OK", res.Severity, res.Status)
	}
	if res.IsResolved {
		t.Error("masked secret should not materialize plaintext")
	}

	revealed := diagnoseResolver(t, true, fixedPrivateKeyResolver{})
	got := revealed.Evaluate("secret://production/database_password", "")
	if !got.IsResolved || got.Value != "database-password" {
		t.Errorf("revealed secret = %+v, want resolved plaintext", got)
	}
}

// TestDiagnoseDanglingReference verifies a reference with no stored value is an
// error classified as SECRET_NOT_FOUND.
func TestDiagnoseDanglingReference(t *testing.T) {
	t.Parallel()

	r := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := r.Evaluate("secret://production/missing", "")
	if res.Severity != severity.Error ||
		res.Status != status.SecretReferenceNotFound {
		t.Errorf("status = %s/%s, want error/SECRET_REFERENCE_NOT_FOUND",
			res.Severity, res.Status)
	}
}

// TestDiagnoseUnavailableKey verifies a reference whose group has no private key
// is a warning classified as PRIVATE_KEY_UNAVAILABLE, never an abort.
func TestDiagnoseUnavailableKey(t *testing.T) {
	t.Parallel()

	r := diagnoseResolver(t, false, newPrivateKeyTestResolver())
	res := r.Evaluate("secret://production/database_password", "")
	if res.Severity != severity.Warn ||
		res.Status != status.PrivateKeyIsUnavailable {
		t.Errorf(
			"status = %s/%s, want warning/PRIVATE_KEY_UNAVAILABLE",
			res.Severity, res.Status,
		)
	}
}

// TestDiagnoseEscapedReference verifies a backslash-escaped reference is an ok
// config value carrying its unescaped literal, materialized only under reveal.
func TestDiagnoseEscapedReference(t *testing.T) {
	t.Parallel()

	const escaped = `\secret://production/database_password`

	masked := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := masked.Evaluate(escaped, "")
	if res.Severity != severity.OK {
		t.Errorf("status = %s/%s, want ok", res.Severity, res.Status)
	}
	if res.IsResolved {
		t.Error("masked escaped reference should not materialize plaintext")
	}

	revealed := diagnoseResolver(t, true, fixedPrivateKeyResolver{})
	got := revealed.Evaluate(escaped, "")
	if !got.IsResolved || got.Value != "secret://production/database_password" {
		t.Errorf("revealed escaped reference = %+v, want the unescaped literal", got)
	}
}

// TestDiagnoseInvalidReference verifies malformed reference grammar is an error
// classified as INVALID_REFERENCE.
func TestDiagnoseInvalidReference(t *testing.T) {
	t.Parallel()

	r := diagnoseResolver(t, false, fixedPrivateKeyResolver{})
	res := r.Evaluate("secret://database_password", "")
	if res.Severity != severity.Error ||
		res.Status != status.InvalidSecretReference {
		t.Errorf("status = %s/%s, want error/INVALID_SECRET_REFERENCE",
			res.Severity, res.Status)
	}
}

// TestDiagnoseNotEncrypted verifies a stored value that is not an envelope is an
// error classified as NOT_ENCRYPTED.
func TestDiagnoseNotEncrypted(t *testing.T) {
	t.Parallel()

	r := diagnoseRawResolver(t, "plain-not-an-envelope")
	res := r.Evaluate("secret://production/database_password", "")
	if res.Severity != severity.Error || res.Status != status.SecretIsNotEncrypted {
		t.Errorf("status = %s/%s, want error/NOT_ENCRYPTED", res.Severity, res.Status)
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
	res := r.Evaluate("secret://production/database_password", "")
	if res.Severity != severity.Error ||
		res.Status != status.SecretAlgorithmMismatch {
		t.Errorf("status = %s/%s, want error/SECRET_ALGORITHM_MISMATCH",
			res.Severity, res.Status)
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

	res := r.Evaluate("secret://production/database_password", "")
	if res.Severity != severity.Error || res.Status != status.PrivateKeyIsInvalid {
		t.Errorf("status = %s/%s, want error/INVALID_PRIVATE_KEY", res.Severity, res.Status)
	}
}
