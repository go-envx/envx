package secrets_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

// countingDecrypter records decrypt calls.
type countingDecrypter struct {
	// Decrypter performs the real decryption.
	secrets.Decrypter
	// calls counts Decrypt calls.
	calls int
}

// Decrypt counts the call and delegates.
func (d *countingDecrypter) Decrypt(
	ciphertext []byte, privateKey string,
) (string, error) {
	d.calls++
	return d.Decrypter.Decrypt(ciphertext, privateKey)
}

// newResolver returns a resolver over the fixture with a counting decrypter.
func newResolver(
	t *testing.T, f *fixture, reveal bool,
) (*secrets.Resolver, *countingDecrypter) {
	t.Helper()
	decrypter := &countingDecrypter{Decrypter: f.cipher}
	resolver, err := secrets.NewResolver(f.store, f.keys, decrypter, reveal)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	return resolver, decrypter
}

// TestResolverResolve verifies passthrough, escaping, masking, revealing, and
// malformed references.
func TestResolverResolve(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")
	f.set(t, "api", "TOKEN", "s3cr3t")

	tests := []struct {
		name    string
		raw     string
		reveal  bool
		want    string
		wantErr bool
	}{
		{"plain value", "postgres://db", true, "postgres://db", false},
		{"escaped literal", `\secret://api/TOKEN`, true, "secret://api/TOKEN", false},
		{"masked", "secret://API/TOKEN", false, "secret://api/TOKEN", false},
		{"masked dangling", "secret://api/MISSING", false, "secret://api/MISSING", false},
		{"revealed", "secret://API/TOKEN", true, "s3cr3t", false},
		{"revealed dangling", "secret://api/MISSING", true, "", true},
		{"malformed", "secret://api", false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, _ := newResolver(t, f, tt.reveal)
			got, err := resolver.Resolve(tt.raw, "dev")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Resolve = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestResolverMaskingNeverReadsStore verifies masked resolution succeeds even
// when the store cannot be read.
func TestResolverMaskingNeverReadsStore(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.store.readErr = errors.New("store unavailable")
	resolver, _ := newResolver(t, f, false)

	got, err := resolver.Resolve("secret://api/TOKEN", "")
	if err != nil || got != "secret://api/TOKEN" {
		t.Errorf("Resolve = %q, %v", got, err)
	}
}

// TestResolverEvaluate verifies each record problem maps to its diagnostic
// code and default severity.
func TestResolverEvaluate(t *testing.T) {
	t.Parallel()
	f := newFixture(t, secrets.Group{
		Name: "legacy",
		Values: map[string]string{
			"PLAIN": "value",
			"OTHER": "encrypted-naclbox:AA",
		},
	})
	f.generate(t, "api")
	f.generate(t, "nokey")
	f.set(t, "api", "TOKEN", "s3cr3t")
	f.set(t, "nokey", "TOKEN", "v")
	delete(f.keys.byPublic, "nokey")

	tests := []struct {
		name  string
		raw   string
		code  diagnostic.Code
		level severity.Level
	}{
		{"config", "plain", "", severity.None},
		{"invalid", "secret://api", diagnostic.InvalidSecretReference, severity.Error},
		{"missing", "secret://api/NONE", diagnostic.SecretReferenceNotFound, severity.Error},
		{
			"plaintext", "secret://legacy/PLAIN",
			diagnostic.SecretIsNotEncrypted, severity.Error,
		},
		{
			"algorithm", "secret://legacy/OTHER",
			diagnostic.SecretAlgorithmMismatch, severity.Error,
		},
		{
			"no key", "secret://nokey/TOKEN",
			diagnostic.PrivateKeyIsUnavailable, severity.Warn,
		},
		{"ok", "secret://api/TOKEN", "", severity.None},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, _ := newResolver(t, f, false)
			eval := resolver.Evaluate(tt.raw, "")
			if eval.Diagnostic.Code != tt.code || eval.Diagnostic.Level != tt.level {
				t.Errorf("Diagnostic = %+v, want %s/%s", eval.Diagnostic, tt.code, tt.level)
			}
		})
	}
}

// TestResolverEvaluateMasksWithoutDecrypting verifies masked evaluation never
// decrypts while revealed evaluation returns plaintext.
func TestResolverEvaluateMasksWithoutDecrypting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")
	f.set(t, "api", "TOKEN", "s3cr3t")

	masked, decrypter := newResolver(t, f, false)
	eval := masked.Evaluate("secret://api/TOKEN", "")
	want := value.Evaluation{Kind: value.Secret, Value: "secret://api/TOKEN"}
	if eval != want {
		t.Errorf("masked Evaluate = %+v, want %+v", eval, want)
	}
	if decrypter.calls != 0 {
		t.Errorf("masked Evaluate decrypted %d times", decrypter.calls)
	}

	revealing, _ := newResolver(t, f, true)
	eval = revealing.Evaluate("secret://api/TOKEN", "")
	want = value.Evaluation{Kind: value.Secret, Value: "s3cr3t", Resolved: true}
	if eval != want {
		t.Errorf("revealed Evaluate = %+v, want %+v", eval, want)
	}
}

// TestResolverEvaluateWrongKey verifies a key that cannot decrypt reports an
// invalid private key.
func TestResolverEvaluateWrongKey(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	publicKey := f.generate(t, "api")
	other := f.generate(t, "other")
	f.set(t, "api", "TOKEN", "s3cr3t")
	f.keys.byPublic["api"][publicKey] = f.keys.byPublic["other"][other]

	resolver, _ := newResolver(t, f, true)
	eval := resolver.Evaluate("secret://api/TOKEN", "")
	if eval.Diagnostic.Code != diagnostic.PrivateKeyIsInvalid || eval.Resolved {
		t.Errorf("Evaluate = %+v, want PRIVATE_KEY_IS_INVALID", eval)
	}
}
