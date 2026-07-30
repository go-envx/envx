package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/go-envx/envx/app/internal/cipher"
)

const encryptedCiphertextPrefix = "encrypted-"

// -------------------------------------------------------------------------------------

// encodeCiphertext wraps native cipher bytes in the single-line storage format
// owned by secrets. The algorithm is stored beside the base64url payload so
// future algorithms can coexist in one store.
func encodeCiphertext(algorithm cipher.Algorithm, payload []byte) (string, error) {
	if err := validateAlgorithm(algorithm); err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("ciphertext payload is empty")
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf(
		"%s%s:%s", encryptedCiphertextPrefix, algorithm, encoded,
	), nil
}

// -------------------------------------------------------------------------------------

// decodeCiphertext unwraps a stored value into its algorithm and native cipher
// bytes. It accepts only the canonical two-part, unpadded base64url format.
func decodeCiphertext(value string) (cipher.Algorithm, []byte, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", nil, errors.New(
			"invalid ciphertext envelope (want encrypted-{algorithm}:{payload})",
		)
	}

	algorithmName, found := strings.CutPrefix(parts[0], encryptedCiphertextPrefix)
	if !found {
		return "", nil, errors.New(
			"invalid ciphertext envelope (want encrypted-{algorithm}:{payload})",
		)
	}
	algorithm := cipher.Algorithm(algorithmName)
	if err := validateAlgorithm(algorithm); err != nil {
		return "", nil, err
	}
	if parts[1] == "" {
		return "", nil, fmt.Errorf("ciphertext payload is empty")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, fmt.Errorf("decode ciphertext payload: %w", err)
	}
	if len(payload) == 0 {
		return "", nil, fmt.Errorf("ciphertext payload is empty")
	}

	return algorithm, payload, nil
}

// -------------------------------------------------------------------------------------

// validateAlgorithm ensures an algorithm identifier cannot change the envelope
// grammar or introduce whitespace into a stored scalar.
func validateAlgorithm(algorithm cipher.Algorithm) error {
	value := string(algorithm)
	if value == "" {
		return fmt.Errorf("ciphertext algorithm is empty")
	}
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.Contains(value, ":") {
		return fmt.Errorf("invalid ciphertext algorithm %q", value)
	}
	return nil
}
