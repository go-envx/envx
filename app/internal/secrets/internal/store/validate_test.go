package store

import "testing"

// TestOpenRejectsMalformedCiphertext verifies plaintext remains accepted while
// values using the encrypted marker must contain a valid envelope.
func TestOpenRejectsMalformedCiphertext(t *testing.T) {
	t.Parallel()

	valid := "secrets:\n  production:\n    value: encrypted-age:YWJj\n"
	if _, err := Open(writeDocument(t, valid)); err != nil {
		t.Fatalf("Open() valid ciphertext error = %v", err)
	}

	invalid := "secrets:\n  production:\n    value: encrypted-age:!!!\n"
	if _, err := Open(writeDocument(t, invalid)); err == nil {
		t.Error("Open() accepted malformed ciphertext")
	}
}
