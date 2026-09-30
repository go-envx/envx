package filestore

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestParseDocumentEmptyAndMalformed verifies empty documents produce an empty document
// and malformed YAML, unknown fields, or invalid document shapes are rejected.
func TestParseDocumentEmptyAndMalformed(t *testing.T) {
	t.Parallel()

	doc, err := parseDocument([]byte(""))
	if err != nil {
		t.Fatalf("parseDocument(\"\") error = %v", err)
	}
	if got := doc.secrets(); len(got) != 0 {
		t.Fatalf("empty document has %d secrets, want 0", len(got))
	}

	for _, body := range []string{
		"{",
		"secrets: [not-a-mapping]",
		"unknown_field: value",
		"public_keys:\n  prod: age1...",
	} {
		if _, err := parseDocument([]byte(body)); err == nil {
			t.Errorf("parseDocument(%q) succeeded", body)
		}
	}
}

// TestParseDocumentAndRead verifies public keys, plaintext values, ciphertext values,
// case-insensitive groups, and document ordering.
func TestParseDocumentAndRead(t *testing.T) {
	t.Parallel()

	doc, err := parseDocument([]byte("public-keys:\n" +
		"  Production: age-public-key:age1production\n" +
		"secrets:\n" +
		"  Production:\n" +
		"    first: plaintext\n" +
		"    second: encrypted-age:YWJj\n" +
		"  shared:\n" +
		"    token: shared-value\n"))
	if err != nil {
		t.Fatalf("parseDocument() error = %v", err)
	}

	if got, ok := doc.publicKey("production"); !ok ||
		got != "age-public-key:age1production" {
		t.Errorf("publicKey() = %q, %v", got, ok)
	}

	sec, ok := doc.secret("PRODUCTION", "second")
	if !ok || sec.Group != "Production" || sec.Key != "second" ||
		sec.Value != "encrypted-age:YWJj" {
		t.Errorf("secret() = %#v, %v", sec, ok)
	}

	allSecs := doc.secrets()
	if len(allSecs) != 3 {
		t.Fatalf("secrets() length = %d, want 3", len(allSecs))
	}
	wantKeys := []string{"first", "second", "token"}
	for i, want := range wantKeys {
		if allSecs[i].Key != want {
			t.Errorf("secrets()[%d].Key = %q, want %q", i, allSecs[i].Key, want)
		}
	}
}

// TestPublicKeyRejectsNonStringScalar verifies public-key reads enforce the
// same string-node invariant as the rest of the document API.
func TestPublicKeyRejectsNonStringScalar(t *testing.T) {
	t.Parallel()

	var root yaml.Node
	if err := yaml.Unmarshal(
		[]byte("public-keys:\n  production: 123\n"),
		&root,
	); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	doc := &document{root: root}
	if got, ok := doc.publicKey("production"); ok {
		t.Errorf("publicKey() = %q, %v; want empty, false", got, ok)
	}
}

// TestWrites verifies additions, updates, and deletion of stored secrets and
// public keys across an encode and reload.
func TestWrites(t *testing.T) {
	t.Parallel()

	input := []byte("secrets:\n  production:\n    first: one\n    second: two\n")
	doc, err := parseDocument(input)
	if err != nil {
		t.Fatalf("parseDocument() error = %v", err)
	}
	if err := doc.setPublicKey("production", "age-public-key:age1prod"); err != nil {
		t.Fatalf("setPublicKey() add error = %v", err)
	}
	if err := doc.setPublicKey(
		"PRODUCTION", "age-public-key:age1updated",
	); err != nil {
		t.Fatalf("setPublicKey() update error = %v", err)
	}
	if err := doc.setSecret("PRODUCTION", "first", "updated"); err != nil {
		t.Fatalf("setSecret() update error = %v", err)
	}
	if err := doc.setSecret("shared", "token", "value"); err != nil {
		t.Fatalf("setSecret() group add error = %v", err)
	}
	if err := doc.setSecret("production", "third", "three"); err != nil {
		t.Fatalf("setSecret() entry add error = %v", err)
	}
	if deleted, err := doc.deleteSecret(
		"production", "second",
	); err != nil || !deleted {
		t.Fatalf("deleteSecret() = %v, %v; want true, nil", deleted, err)
	}
	if deleted, err := doc.deleteSecret(
		"production", "missing",
	); err != nil || deleted {
		t.Fatalf("deleteSecret() missing = %v, %v; want false, nil", deleted, err)
	}

	encoded, err := doc.encode(0)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	reloaded, err := parseDocument(encoded)
	if err != nil {
		t.Fatalf("parseDocument() after encode error = %v", err)
	}
	if got, ok := reloaded.publicKey("production"); !ok ||
		got != "age-public-key:age1updated" {
		t.Errorf("publicKey() after encode = %q, %v", got, ok)
	}
	if got, ok := reloaded.secret("production", "first"); !ok || got.Value != "updated" {
		t.Errorf("secret() after encode = %#v, %v", got, ok)
	}
	if _, ok := reloaded.secret("production", "second"); ok {
		t.Error("deleted secret is still present")
	}
	if _, ok := reloaded.secret("shared", "token"); !ok {
		t.Error("new group secret is absent")
	}
}

// TestDeleteRemovesEmptiedGroup verifies deleting a group's last secret drops
// the empty group mapping while leaving other groups and the public key intact.
func TestDeleteRemovesEmptiedGroup(t *testing.T) {
	t.Parallel()

	input := []byte("public-keys:\n  dev: age-public-key:age1dev\n" +
		"secrets:\n  dev:\n    only: value\n  shared:\n    token: kept\n")
	doc, err := parseDocument(input)
	if err != nil {
		t.Fatalf("parseDocument() error = %v", err)
	}
	if deleted, err := doc.deleteSecret("dev", "only"); err != nil || !deleted {
		t.Fatalf("deleteSecret() = %v, %v; want true, nil", deleted, err)
	}
	encoded, err := doc.encode(0)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}

	if output := string(encoded); strings.Contains(output, "dev: {}") {
		t.Fatalf("emptied group left as an empty mapping:\n%s", output)
	}

	reloaded, err := parseDocument(encoded)
	if err != nil {
		t.Fatalf("parseDocument() after encode error = %v", err)
	}
	if got, ok := reloaded.publicKey("dev"); !ok || got != "age-public-key:age1dev" {
		t.Errorf("publicKey(dev) after delete = %q, %v; want kept", got, ok)
	}
	if _, ok := reloaded.secret("shared", "token"); !ok {
		t.Error("unrelated group secret was removed")
	}
}

// TestDeleteRemovesEmptiedSecretsBlock verifies deleting the last secret across
// all groups drops the whole secrets block while leaving public keys intact.
func TestDeleteRemovesEmptiedSecretsBlock(t *testing.T) {
	t.Parallel()

	input := []byte("public-keys:\n  dev: age-public-key:age1dev\n" +
		"secrets:\n  dev:\n    only: value\n")
	doc, err := parseDocument(input)
	if err != nil {
		t.Fatalf("parseDocument() error = %v", err)
	}
	if deleted, err := doc.deleteSecret("dev", "only"); err != nil || !deleted {
		t.Fatalf("deleteSecret() = %v, %v; want true, nil", deleted, err)
	}
	encoded, err := doc.encode(0)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}

	if output := string(encoded); strings.Contains(output, "secrets:") {
		t.Fatalf("emptied secrets block was not removed:\n%s", output)
	}

	reloaded, err := parseDocument(encoded)
	if err != nil {
		t.Fatalf("parseDocument() after encode error = %v", err)
	}
	if got, ok := reloaded.publicKey("dev"); !ok || got != "age-public-key:age1dev" {
		t.Errorf("publicKey(dev) after delete = %q, %v; want kept", got, ok)
	}
	if len(reloaded.secrets()) != 0 {
		t.Fatalf("secrets() after delete has %d entries, want 0", len(reloaded.secrets()))
	}
}

// TestParseDocumentRejectsMalformedCiphertext verifies plaintext remains accepted while
// values using the encrypted marker must contain a valid envelope.
func TestParseDocumentRejectsMalformedCiphertext(t *testing.T) {
	t.Parallel()

	valid := []byte("secrets:\n  production:\n    value: encrypted-age:YWJj\n")
	if _, err := parseDocument(valid); err != nil {
		t.Fatalf("parseDocument() valid ciphertext error = %v", err)
	}

	invalid := []byte("secrets:\n  production:\n    value: encrypted-age:!!!\n")
	if _, err := parseDocument(invalid); err == nil {
		t.Error("parseDocument() accepted malformed ciphertext")
	}
}

// TestDocumentRejectsInvalidIdentifiers verifies group and key syntax validation
// across parse and mutation operations.
func TestDocumentRejectsInvalidIdentifiers(t *testing.T) {
	t.Parallel()

	for _, invalidGroup := range []string{
		"public-keys:\n  \"\": age-public-key:age1dev\n",
		"public-keys:\n  \"group with spaces\": age-public-key:age1dev\n",
		"public-keys:\n  \"group/slash\": age-public-key:age1dev\n",
		"secrets:\n  \"group/slash\":\n    key: val\n",
		"secrets:\n  group:\n    \"key/with/slash\": val\n",
		"secrets:\n  group:\n    \"\": val\n",
	} {
		if _, err := parseDocument([]byte(invalidGroup)); err == nil {
			t.Errorf("parseDocument(%q) accepted invalid identifier", invalidGroup)
		}
	}

	doc, err := parseDocument(nil)
	if err != nil {
		t.Fatalf("parseDocument(nil): %v", err)
	}
	if err := doc.setPublicKey("bad/group", "pubkey"); err == nil {
		t.Error("setPublicKey() accepted invalid group name")
	}
	if err := doc.setSecret("bad/group", "key", "val"); err == nil {
		t.Error("setSecret() accepted invalid group name")
	}
	if err := doc.setSecret("group", "bad/key", "val"); err == nil {
		t.Error("setSecret() accepted invalid key name")
	}
	if _, err := doc.deleteSecret("bad/group", "key"); err == nil {
		t.Error("deleteSecret() accepted invalid group name")
	}
	if _, err := doc.deleteSecret("group", "bad/key"); err == nil {
		t.Error("deleteSecret() accepted invalid key name")
	}
}
