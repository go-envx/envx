package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// envelopePrefix marks an algorithm-tagged ciphertext.
const envelopePrefix = "encrypted-"

// errInvalidEnvelope marks a value with the envelope prefix but a broken shape.
var errInvalidEnvelope = errors.New(
	"invalid ciphertext envelope (want " + envelopePrefix + "{algorithm}:{payload})",
)

// envelope is a decoded algorithm-tagged ciphertext.
type envelope struct {
	// algorithm names the cipher that sealed payload.
	algorithm string
	// payload is the native ciphertext.
	payload []byte
}

// encode renders the envelope in its stored form.
func (e envelope) encode() (string, error) {
	if err := validateAlgorithm(e.algorithm); err != nil {
		return "", err
	}
	if len(e.payload) == 0 {
		return "", errors.New("ciphertext payload is empty")
	}
	encoded := base64.RawURLEncoding.EncodeToString(e.payload)
	return envelopePrefix + e.algorithm + ":" + encoded, nil
}

// sealedBy reports whether the envelope was produced by the named cipher.
func (e envelope) sealedBy(algorithm string) bool {
	return e.algorithm == algorithm
}

// ValidateEnvelope rejects a value that carries the envelope prefix but is not
// a well-formed envelope. Values without the prefix are not envelopes and pass.
func ValidateEnvelope(stored string) error {
	if !isEnvelope(stored) {
		return nil
	}
	_, err := decodeEnvelope(stored)
	return err
}

// isEnvelope reports whether a stored value claims the ciphertext format.
func isEnvelope(stored string) bool {
	return strings.HasPrefix(stored, envelopePrefix)
}

// decodeEnvelope parses a stored value. It accepts only the canonical
// two-part, unpadded base64url format.
func decodeEnvelope(stored string) (envelope, error) {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return envelope{}, errInvalidEnvelope
	}
	algorithm, found := strings.CutPrefix(parts[0], envelopePrefix)
	if !found {
		return envelope{}, errInvalidEnvelope
	}
	if err := validateAlgorithm(algorithm); err != nil {
		return envelope{}, err
	}
	if parts[1] == "" {
		return envelope{}, errors.New("ciphertext payload is empty")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return envelope{}, fmt.Errorf("decode ciphertext payload: %w", err)
	}
	if len(payload) == 0 {
		return envelope{}, errors.New("ciphertext payload is empty")
	}
	return envelope{algorithm: algorithm, payload: payload}, nil
}

// openEnvelope decodes a stored value sealed by the configured cipher and
// describes failures for ref.
func openEnvelope(ref Reference, stored, configured string) ([]byte, error) {
	env, err := decodeEnvelope(stored)
	if err != nil {
		return nil, fmt.Errorf(
			"secret %q in group %q is not encrypted: %w", ref.Key, ref.Group, err,
		)
	}
	if !env.sealedBy(configured) {
		return nil, fmt.Errorf(
			"secret %q in group %q uses algorithm %q, "+
				"but the configured cipher is %q",
			ref.Key, ref.Group, env.algorithm, configured,
		)
	}
	return env.payload, nil
}

// validateAlgorithm ensures an algorithm identifier cannot change the envelope
// grammar or introduce whitespace into a stored scalar.
func validateAlgorithm(algorithm string) error {
	if algorithm == "" {
		return errors.New("ciphertext algorithm is empty")
	}
	if strings.IndexFunc(algorithm, unicode.IsSpace) >= 0 ||
		strings.Contains(algorithm, ":") {
		return fmt.Errorf("invalid ciphertext algorithm %q", algorithm)
	}
	return nil
}
