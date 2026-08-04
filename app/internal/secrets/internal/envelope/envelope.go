package envelope

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/go-envx/envx/app/internal/cipher"
)

const ciphertextPrefix = "encrypted-"

// -------------------------------------------------------------------------------------

// Encode wraps native cipher bytes in the algorithm-tagged storage format.
func Encode(algorithm cipher.Algorithm, payload []byte) (string, error) {
	if err := validateAlgorithm(algorithm); err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", errors.New("ciphertext payload is empty")
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf(
		"%s%s:%s", ciphertextPrefix, algorithm, encoded,
	), nil
}

// -------------------------------------------------------------------------------------

// Decode unwraps a stored value into its algorithm and native cipher bytes. It
// accepts only the canonical two-part, unpadded base64url format.
func Decode(value string) (cipher.Algorithm, []byte, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", nil, invalidEnvelopeError()
	}

	algorithmName, found := strings.CutPrefix(parts[0], ciphertextPrefix)
	if !found {
		return "", nil, invalidEnvelopeError()
	}
	algorithm := cipher.Algorithm(algorithmName)
	if err := validateAlgorithm(algorithm); err != nil {
		return "", nil, err
	}
	if parts[1] == "" {
		return "", nil, errors.New("ciphertext payload is empty")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, fmt.Errorf("decode ciphertext payload: %w", err)
	}
	if len(payload) == 0 {
		return "", nil, errors.New("ciphertext payload is empty")
	}

	return algorithm, payload, nil
}

// -------------------------------------------------------------------------------------

// Validate checks whether value is a valid algorithm-tagged ciphertext envelope.
func Validate(value string) error {
	_, _, err := Decode(value)
	return err
}

// -------------------------------------------------------------------------------------

// IsCiphertext reports whether value claims the ciphertext envelope format.
func IsCiphertext(value string) bool {
	return strings.HasPrefix(value, ciphertextPrefix)
}

// -------------------------------------------------------------------------------------

// validateAlgorithm ensures an algorithm identifier cannot change the envelope
// grammar or introduce whitespace into a stored scalar.
func validateAlgorithm(algorithm cipher.Algorithm) error {
	value := string(algorithm)
	if value == "" {
		return errors.New("ciphertext algorithm is empty")
	}
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.Contains(value, ":") {
		return fmt.Errorf("invalid ciphertext algorithm %q", value)
	}
	return nil
}

// -------------------------------------------------------------------------------------

// invalidEnvelopeError returns the stable malformed-envelope error shared by
// all callers that parse stored ciphertext values.
func invalidEnvelopeError() error {
	return errors.New(
		"invalid ciphertext envelope (want " + ciphertextPrefix + "{algorithm}:{payload})",
	)
}
