package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// defaultCiphertextPrefix marks an algorithm-tagged ciphertext.
const defaultCiphertextPrefix = "encrypted-"

// envelopeParams supplies configuration for the ciphertext envelope codec.
type envelopeParams struct {
	// Prefix specifies the envelope prefix. Defaults to "encrypted-" if empty.
	Prefix string
}

// envelopeCodec owns encoding, decoding, and validating ciphertext envelopes.
type envelopeCodec struct {
	prefix string
}

// newEnvelopeCodec constructs an envelope codec with the provided configuration.
func newEnvelopeCodec(params envelopeParams) *envelopeCodec {
	prefix := params.Prefix
	if prefix == "" {
		prefix = defaultCiphertextPrefix
	}
	return &envelopeCodec{prefix: prefix}
}

// encode wraps native cipher bytes in the algorithm-tagged storage format.
func (c *envelopeCodec) encode(algorithm string, payload []byte) (string, error) {
	if err := c.validateAlgorithm(algorithm); err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", errors.New("ciphertext payload is empty")
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf("%s%s:%s", c.prefix, algorithm, encoded), nil
}

// decode unwraps a stored value into its algorithm and native cipher bytes. It
// accepts only the canonical two-part, unpadded base64url format.
func (c *envelopeCodec) decode(
	value string,
) (algorithm string, payload []byte, err error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", nil, c.invalidEnvelopeError()
	}

	algorithmName, found := strings.CutPrefix(parts[0], c.prefix)
	if !found {
		return "", nil, c.invalidEnvelopeError()
	}
	if err := c.validateAlgorithm(algorithmName); err != nil {
		return "", nil, err
	}
	if parts[1] == "" {
		return "", nil, errors.New("ciphertext payload is empty")
	}

	payload, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, fmt.Errorf("decode ciphertext payload: %w", err)
	}
	if len(payload) == 0 {
		return "", nil, errors.New("ciphertext payload is empty")
	}

	return algorithmName, payload, nil
}

// validate checks whether value is a valid algorithm-tagged ciphertext envelope.
func (c *envelopeCodec) validate(value string) error {
	_, _, err := c.decode(value)
	return err
}

// isCiphertext reports whether value claims the ciphertext envelope format.
func (c *envelopeCodec) isCiphertext(value string) bool {
	return strings.HasPrefix(value, c.prefix)
}

// validateAlgorithm ensures an algorithm identifier cannot change the envelope
// grammar or introduce whitespace into a stored scalar.
func (c *envelopeCodec) validateAlgorithm(algorithm string) error {
	if algorithm == "" {
		return errors.New("ciphertext algorithm is empty")
	}
	if strings.IndexFunc(algorithm, unicode.IsSpace) >= 0 ||
		strings.Contains(algorithm, ":") {
		return fmt.Errorf("invalid ciphertext algorithm %q", algorithm)
	}
	return nil
}

// invalidEnvelopeError returns the stable malformed-envelope error shared by
// all callers that parse stored ciphertext values.
func (c *envelopeCodec) invalidEnvelopeError() error {
	return errors.New(
		"invalid ciphertext envelope (want " + c.prefix + "{algorithm}:{payload})",
	)
}
