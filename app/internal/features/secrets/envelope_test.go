package secrets

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/resources/cipher"
)

// TestEnvelopeRoundTrip verifies algorithm metadata and native bytes survive the
// single-line storage encoding unchanged.
func TestEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	codec := newEnvelopeCodec(envelopeParams{})

	payload := []byte("age-encryption.org/v1\n\x00\xff")
	value, err := codec.encode(string(cipher.Age), payload)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if strings.ContainsAny(value, "\r\n") {
		t.Fatalf("encode() returned a multiline value: %q", value)
	}
	if !strings.HasPrefix(value, "encrypted-age:") {
		t.Fatalf("encode() = %q, want encrypted-age prefix", value)
	}

	algorithm, decoded, err := codec.decode(value)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if algorithm != string(cipher.Age) {
		t.Fatalf("decode() algorithm = %q, want %q", algorithm, cipher.Age)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decode() payload = %x, want %x", decoded, payload)
	}
}

// TestEnvelopeCustomPrefix verifies the envelope codec respects a customized prefix.
func TestEnvelopeCustomPrefix(t *testing.T) {
	t.Parallel()

	codec := newEnvelopeCodec(envelopeParams{
		Prefix: "custom-prefix-",
	})

	payload := []byte("test-payload")
	encoded, err := codec.encode("age", payload)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if !strings.HasPrefix(encoded, "custom-prefix-age:") {
		t.Fatalf("encode() = %q, want custom-prefix-age: prefix", encoded)
	}
	if !codec.isCiphertext(encoded) {
		t.Fatalf("isCiphertext() = false for custom prefix encoded value")
	}

	alg, decoded, err := codec.decode(encoded)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if alg != "age" || !bytes.Equal(decoded, payload) {
		t.Fatalf("decode() got alg=%s payload=%s", alg, string(decoded))
	}
}

// TestEnvelopeComposesWithAge verifies the storage envelope can carry and restore a
// real age ciphertext without involving armor.
func TestEnvelopeComposesWithAge(t *testing.T) {
	t.Parallel()

	codec := newEnvelopeCodec(envelopeParams{})

	selected, err := cipher.New(cipher.Params{
		Algorithm: cipher.Age,
		Options:   cipher.AgeOptions{},
	})
	if err != nil {
		t.Fatalf("cipher.New() error = %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	nativeCiphertext, err := selected.Encrypt("composed-secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	storedValue, err := codec.encode(string(cipher.Age), nativeCiphertext)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}

	algorithm, decodedCiphertext, err := codec.decode(storedValue)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	selected, err = cipher.New(cipher.Params{
		Algorithm: cipher.Algorithm(algorithm),
		Options:   cipher.AgeOptions{},
	})
	if err != nil {
		t.Fatalf("cipher.New(decoded algorithm) error = %v", err)
	}
	plaintext, err := selected.Decrypt(decodedCiphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "composed-secret" {
		t.Fatalf("Decrypt() = %q, want %q", plaintext, "composed-secret")
	}
}

// TestEnvelopeComposesWithNaClBox verifies a second algorithm uses the same envelope
// without special-case parsing.
func TestEnvelopeComposesWithNaClBox(t *testing.T) {
	t.Parallel()

	codec := newEnvelopeCodec(envelopeParams{})

	selected, err := cipher.New(cipher.Params{
		Algorithm: cipher.NaClBox,
		Options:   cipher.NaClBoxOptions{},
	})
	if err != nil {
		t.Fatalf("cipher.New() error = %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	nativeCiphertext, err := selected.Encrypt("composed-secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	storedValue, err := codec.encode(string(cipher.NaClBox), nativeCiphertext)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if !strings.HasPrefix(storedValue, "encrypted-nacl-box:") {
		t.Fatalf("encode() = %q, want encrypted-nacl-box prefix", storedValue)
	}

	algorithm, decodedCiphertext, err := codec.decode(storedValue)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	selected, err = cipher.New(cipher.Params{
		Algorithm: cipher.Algorithm(algorithm),
		Options:   cipher.NaClBoxOptions{},
	})
	if err != nil {
		t.Fatalf("cipher.New(decoded algorithm) error = %v", err)
	}
	plaintext, err := selected.Decrypt(decodedCiphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "composed-secret" {
		t.Fatalf("Decrypt() = %q, want %q", plaintext, "composed-secret")
	}
}

// TestEnvelopeDecodeRejectsMalformedEnvelopes verifies malformed structure, algorithms,
// and payloads are rejected.
func TestEnvelopeDecodeRejectsMalformedEnvelopes(t *testing.T) {
	t.Parallel()

	codec := newEnvelopeCodec(envelopeParams{})

	tests := []string{
		"",
		"age",
		"age:payload:extra",
		":cGF5bG9hZA",
		"bad algorithm:cGF5bG9hZA",
		"age:",
		"age:not base64",
		"age:cGF5bG9hZA=",
		"encrypted-:cGF5bG9hZA",
		"encrypted-age:not base64",
		"encrypted-age:cGF5bG9hZA=",
	}
	for _, value := range tests {
		if _, _, err := codec.decode(value); err == nil {
			t.Errorf("decode(%q) succeeded", value)
		}
	}
}

// TestEnvelopeEncodeRejectsInvalidInputs verifies invalid algorithm names and empty
// native payloads cannot enter the store format.
func TestEnvelopeEncodeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	codec := newEnvelopeCodec(envelopeParams{})

	algorithms := []string{
		"",
		"bad algorithm",
		"bad:algorithm",
		"bad\nalgorithm",
	}
	for _, algorithm := range algorithms {
		if _, err := codec.encode(algorithm, []byte("payload")); err == nil {
			t.Errorf("encode(%q) succeeded", algorithm)
		}
	}
	if _, err := codec.encode("age", nil); err == nil {
		t.Fatal("encode() accepted an empty payload")
	}
}

// TestEnvelopeIsCiphertext verifies the marker check distinguishes claimed envelopes
// from ordinary plaintext values.
func TestEnvelopeIsCiphertext(t *testing.T) {
	t.Parallel()

	codec := newEnvelopeCodec(envelopeParams{})

	if !codec.isCiphertext("encrypted-age:payload") {
		t.Error("isCiphertext() rejected an envelope marker")
	}
	if codec.isCiphertext("plaintext") {
		t.Error("isCiphertext() accepted plaintext")
	}
}

// TestValidateSecretValue verifies the domain validation function for secret entries.
func TestValidateSecretValue(t *testing.T) {
	t.Parallel()

	if err := ValidateSecretValue("prod", "key", "plaintext"); err != nil {
		t.Fatalf("ValidateSecretValue() rejected plaintext: %v", err)
	}
	if err := ValidateSecretValue("prod", "key", "encrypted-age:YWJj"); err != nil {
		t.Fatalf("ValidateSecretValue() rejected valid envelope: %v", err)
	}
	invalidBase64 := "encrypted-age:invalid base64!!!"
	if err := ValidateSecretValue("prod", "key", invalidBase64); err == nil {
		t.Fatalf("ValidateSecretValue() accepted invalid base64 payload")
	}
	if err := ValidateSecretValue("prod", "key", "encrypted-bad alg:YWJj"); err == nil {
		t.Fatalf("ValidateSecretValue() accepted invalid algorithm name")
	}
}
