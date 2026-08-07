package envelope

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
)

// -------------------------------------------------------------------------------------

// TestRoundTrip verifies algorithm metadata and native bytes survive the
// single-line storage encoding unchanged.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	payload := []byte("age-encryption.org/v1\n\x00\xff")
	value, err := Encode(cipher.Age, payload)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if strings.ContainsAny(value, "\r\n") {
		t.Fatalf("Encode() returned a multiline value: %q", value)
	}
	if !strings.HasPrefix(value, "encrypted-age:") {
		t.Fatalf("Encode() = %q, want encrypted-age prefix", value)
	}

	algorithm, decoded, err := Decode(value)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if algorithm != cipher.Age {
		t.Fatalf("Decode() algorithm = %q, want %q", algorithm, cipher.Age)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("Decode() payload = %x, want %x", decoded, payload)
	}
}

// -------------------------------------------------------------------------------------

// TestComposesWithAge verifies the storage envelope can carry and restore a
// real age ciphertext without involving armor.
func TestComposesWithAge(t *testing.T) {
	t.Parallel()

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
	storedValue, err := Encode(cipher.Age, nativeCiphertext)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	algorithm, decodedCiphertext, err := Decode(storedValue)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	selected, err = cipher.New(cipher.Params{
		Algorithm: algorithm,
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

// -------------------------------------------------------------------------------------

// TestComposesWithNaClBox verifies a second algorithm uses the same envelope
// without special-case parsing.
func TestComposesWithNaClBox(t *testing.T) {
	t.Parallel()

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
	storedValue, err := Encode(cipher.NaClBox, nativeCiphertext)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !strings.HasPrefix(storedValue, "encrypted-nacl-box:") {
		t.Fatalf("Encode() = %q, want encrypted-nacl-box prefix", storedValue)
	}

	algorithm, decodedCiphertext, err := Decode(storedValue)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	selected, err = cipher.New(cipher.Params{
		Algorithm: algorithm,
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

// -------------------------------------------------------------------------------------

// TestDecodeRejectsMalformedEnvelopes verifies malformed structure, algorithms,
// and payloads are rejected.
func TestDecodeRejectsMalformedEnvelopes(t *testing.T) {
	t.Parallel()

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
		if _, _, err := Decode(value); err == nil {
			t.Errorf("Decode(%q) succeeded", value)
		}
	}
}

// -------------------------------------------------------------------------------------

// TestEncodeRejectsInvalidInputs verifies invalid algorithm names and empty
// native payloads cannot enter the store format.
func TestEncodeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	algorithms := []cipher.Algorithm{
		"",
		"bad algorithm",
		"bad:algorithm",
		"bad\nalgorithm",
	}
	for _, algorithm := range algorithms {
		if _, err := Encode(algorithm, []byte("payload")); err == nil {
			t.Errorf("Encode(%q) succeeded", algorithm)
		}
	}
	if _, err := Encode(cipher.Age, nil); err == nil {
		t.Fatal("Encode() accepted an empty payload")
	}
}

// -------------------------------------------------------------------------------------

// TestIsCiphertext verifies the marker check distinguishes claimed envelopes
// from ordinary plaintext values.
func TestIsCiphertext(t *testing.T) {
	t.Parallel()

	if !IsCiphertext("encrypted-age:payload") {
		t.Error("IsCiphertext() rejected an envelope marker")
	}
	if IsCiphertext("plaintext") {
		t.Error("IsCiphertext() accepted plaintext")
	}
}
