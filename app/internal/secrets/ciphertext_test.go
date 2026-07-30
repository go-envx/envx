package secrets

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
)

// -------------------------------------------------------------------------------------

// TestCiphertextEnvelopeRoundTrip verifies algorithm metadata and native bytes
// survive the single-line storage encoding unchanged.
func TestCiphertextEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	payload := []byte("age-encryption.org/v1\n\x00\xff")
	value, err := encodeCiphertext(cipher.Age, payload)
	if err != nil {
		t.Fatalf("encodeCiphertext() error = %v", err)
	}
	if strings.ContainsAny(value, "\r\n") {
		t.Fatalf("encodeCiphertext() returned a multiline value: %q", value)
	}
	if !strings.HasPrefix(value, "encrypted-age:") {
		t.Fatalf("encodeCiphertext() = %q, want encrypted-age prefix", value)
	}

	algorithm, decoded, err := decodeCiphertext(value)
	if err != nil {
		t.Fatalf("decodeCiphertext() error = %v", err)
	}
	if algorithm != cipher.Age {
		t.Fatalf("decodeCiphertext() algorithm = %q, want %q", algorithm, cipher.Age)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decodeCiphertext() payload = %x, want %x", decoded, payload)
	}
}

// -------------------------------------------------------------------------------------

// TestCiphertextEnvelopeComposesWithAge verifies the storage envelope can
// carry and restore a real age ciphertext without involving armor.
func TestCiphertextEnvelopeComposesWithAge(t *testing.T) {
	t.Parallel()

	selected, err := cipher.New(cipher.Age, cipher.AgeOptions{})
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
	storedValue, err := encodeCiphertext(cipher.Age, nativeCiphertext)
	if err != nil {
		t.Fatalf("encodeCiphertext() error = %v", err)
	}

	algorithm, decodedCiphertext, err := decodeCiphertext(storedValue)
	if err != nil {
		t.Fatalf("decodeCiphertext() error = %v", err)
	}
	selected, err = cipher.New(algorithm, cipher.AgeOptions{})
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

// TestCiphertextEnvelopeComposesWithNaClBox verifies a second algorithm uses
// the same single-line storage envelope without special-case parsing.
func TestCiphertextEnvelopeComposesWithNaClBox(t *testing.T) {
	t.Parallel()

	selected, err := cipher.New(cipher.NaClBox, cipher.NaClBoxOptions{})
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
	storedValue, err := encodeCiphertext(cipher.NaClBox, nativeCiphertext)
	if err != nil {
		t.Fatalf("encodeCiphertext() error = %v", err)
	}
	if !strings.HasPrefix(storedValue, "encrypted-nacl-box:") {
		t.Fatalf("encodeCiphertext() = %q, want encrypted-nacl-box prefix", storedValue)
	}

	algorithm, decodedCiphertext, err := decodeCiphertext(storedValue)
	if err != nil {
		t.Fatalf("decodeCiphertext() error = %v", err)
	}
	selected, err = cipher.New(algorithm, cipher.NaClBoxOptions{})
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

// TestDecodeCiphertextRejectsMalformedEnvelopes verifies the envelope parser
// rejects malformed structure, algorithms, and payloads.
func TestDecodeCiphertextRejectsMalformedEnvelopes(t *testing.T) {
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
		if _, _, err := decodeCiphertext(value); err == nil {
			t.Errorf("decodeCiphertext(%q) succeeded", value)
		}
	}
}

// -------------------------------------------------------------------------------------

// TestEncodeCiphertextRejectsInvalidInputs verifies invalid algorithm names
// and empty native payloads cannot enter the store format.
func TestEncodeCiphertextRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	algorithms := []cipher.Algorithm{
		"",
		"bad algorithm",
		"bad:algorithm",
		"bad\nalgorithm",
	}
	for _, algorithm := range algorithms {
		if _, err := encodeCiphertext(algorithm, []byte("payload")); err == nil {
			t.Errorf("encodeCiphertext(%q) succeeded", algorithm)
		}
	}
	if _, err := encodeCiphertext(cipher.Age, nil); err == nil {
		t.Fatal("encodeCiphertext() accepted an empty payload")
	}
}
