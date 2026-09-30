package secrets

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// SecretRecord holds encrypted secret payload details.
type SecretRecord struct {
	Group      string
	Key        string
	Ciphertext string
}

// Algorithm extracts the envelope algorithm from Ciphertext,
// or returns an empty string if the secret is plaintext or the envelope is invalid.
func (r SecretRecord) Algorithm() string {
	codec := newEnvelopeCodec(envelopeParams{})
	if !codec.isCiphertext(r.Ciphertext) {
		return ""
	}
	alg, _, err := codec.decode(r.Ciphertext)
	if err != nil {
		return ""
	}
	return alg
}

// SecretReference identifies one stored secret without carrying its value.
type SecretReference struct {
	Group string
	Key   string
}

// PlaintextResolver lazily supplies one secret plaintext value.
type PlaintextResolver func() (string, error)

// NormalizeGroupName validates a key-group name and returns its canonical form.
func NormalizeGroupName(group string) (string, error) {
	if strings.TrimSpace(group) == "" {
		return "", errors.New("secret group is empty")
	}
	if strings.IndexFunc(group, unicode.IsSpace) >= 0 ||
		strings.ContainsAny(group, "/\r\n=") {
		return "", fmt.Errorf("invalid secret group %q", group)
	}
	return strings.ToLower(group), nil
}

// ValidateSecretGroup validates a key-group name identifier.
func ValidateSecretGroup(group string) error {
	_, err := NormalizeGroupName(group)
	return err
}

// ValidateSecretKey validates the exact key identifier accepted by references
// and the document store.
func ValidateSecretKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("secret key is empty")
	}
	if strings.ContainsAny(key, "/\r\n") {
		return fmt.Errorf("invalid secret key %q", key)
	}
	return nil
}

// ValidateSecretValue accepts plaintext values and checks values that claim
// the encrypted envelope format using the default envelope codec.
func ValidateSecretValue(group, key, value string) error {
	defaultCodec := newEnvelopeCodec(envelopeParams{})
	if !defaultCodec.isCiphertext(value) {
		return nil
	}
	if err := defaultCodec.validate(value); err != nil {
		return fmt.Errorf("secret %q in group %q: %w", key, group, err)
	}
	return nil
}
