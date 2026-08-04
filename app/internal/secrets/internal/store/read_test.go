package store

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------

// TestOpenAndRead verifies public keys, plaintext values, ciphertext values,
// case-insensitive groups, and document ordering.
func TestOpenAndRead(t *testing.T) {
	t.Parallel()

	document, err := Open(writeDocument(t, "public_keys:\n"+
		"  Production: age-public-key:age1production\n"+
		"secrets:\n"+
		"  Production:\n"+
		"    first: plaintext\n"+
		"    second: encrypted-age:YWJj\n"+
		"  shared:\n"+
		"    token: shared-value\n"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if got, ok := document.PublicKey("production"); !ok ||
		got != "age-public-key:age1production" {
		t.Errorf("PublicKey() = %q, %v", got, ok)
	}

	secret, ok := document.Secret("PRODUCTION", "second")
	if !ok || secret.Group != "Production" || secret.Key != "second" ||
		secret.Value != "encrypted-age:YWJj" {
		t.Errorf("Secret() = %#v, %v", secret, ok)
	}

	secrets := document.Secrets()
	if len(secrets) != 3 {
		t.Fatalf("Secrets() length = %d, want 3", len(secrets))
	}
	wantKeys := []string{"first", "second", "token"}
	for i, want := range wantKeys {
		if secrets[i].Key != want {
			t.Errorf("Secrets()[%d].Key = %q, want %q", i, secrets[i].Key, want)
		}
	}
}

// -------------------------------------------------------------------------------------

// TestPublicKeyRejectsNonStringScalar verifies public-key reads enforce the
// same string-node invariant as the rest of the document API.
func TestPublicKeyRejectsNonStringScalar(t *testing.T) {
	t.Parallel()

	var root yaml.Node
	if err := yaml.Unmarshal(
		[]byte("public_keys:\n  production: 123\n"),
		&root,
	); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	document := &Document{root: root}
	if got, ok := document.PublicKey("production"); ok {
		t.Errorf("PublicKey() = %q, %v; want empty, false", got, ok)
	}
}
