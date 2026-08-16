package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
)

// ErrSecretNotFound indicates a reference to a value absent from the store. It
// is the dangling-reference sentinel, distinct from the cipher and private-key
// sentinels a diagnosis reuses for the remaining failure modes.
var ErrSecretNotFound = errors.New("secret not found")

// Status codes reported by Diagnose. Each is a stable identifier, free of
// private-key or resolved-secret material, so machine output stays classifiable.
const (
	// CodeOK marks a value that resolved successfully.
	CodeOK = "OK"
	// CodePrivateKeyUnavailable marks a reference with no private key in context.
	CodePrivateKeyUnavailable = "PRIVATE_KEY_UNAVAILABLE"
	// CodeSecretNotFound marks a dangling reference with no stored value.
	CodeSecretNotFound = "SECRET_NOT_FOUND"
	// CodeInvalidPrivateKey marks a present but malformed or mismatched key.
	CodeInvalidPrivateKey = "INVALID_PRIVATE_KEY"
	// CodeAlgorithmMismatch marks an envelope algorithm the cipher cannot use.
	CodeAlgorithmMismatch = "ALGORITHM_MISMATCH"
	// CodeInvalidReference marks a value whose reference grammar is malformed.
	CodeInvalidReference = "INVALID_REFERENCE"
	// CodeNotEncrypted marks a referenced store value that is not ciphertext.
	CodeNotEncrypted = "NOT_ENCRYPTED"
)

// Diagnose reports a value's kind and dry-run resolution without materializing
// plaintext unless the resolver reveals. It never returns an error; a failure is
// reported through the Resolution's severity and code, and no private-key or
// resolved-secret material appears in Code or Message.
func (r *Resolver) Diagnose(value, _ string) envmerge.Resolution {
	// Unescape a literal that starts with the reserved scheme by dropping only the
	// leading backslash, matching Resolve so its literal is "secret://...".
	if strings.HasPrefix(value, `\`+scheme) {
		return r.configValueResolution(value[1:])
	}
	// Ordinary values are plain configuration.
	if !strings.HasPrefix(value, scheme) {
		return r.configValueResolution(value)
	}

	// Parse the reference; malformed grammar is an error regardless of policy.
	body := strings.TrimPrefix(value, scheme)
	ref, err := splitRef(body)
	if err != nil {
		return envmerge.Resolution{
			Kind:     envmerge.KindSecretReference,
			Severity: envmerge.SeverityError,
			Code:     CodeInvalidReference,
			Message:  err.Error(),
		}
	}
	return r.diagnoseReference(ref)
}

// configValueResolution reports an ok config value, attaching its resolved form
// only when the resolver reveals.
func (r *Resolver) configValueResolution(value string) envmerge.Resolution {
	res := envmerge.Resolution{
		Kind:     envmerge.KindConfigValue,
		Severity: envmerge.SeverityOK,
		Code:     CodeOK,
	}
	if r.reveal {
		res.Resolved = value
		res.HasResolved = true
	}
	return res
}

// diagnoseReference classifies a well-formed reference by attempting decryption
// for status only. It discards any plaintext unless the resolver reveals, and
// maps typed cipher and private-key failures onto stable status codes.
func (r *Resolver) diagnoseReference(ref reference) envmerge.Resolution {
	res := envmerge.Resolution{Kind: envmerge.KindSecretReference}

	ciphertext, ok := r.values[ref]
	if !ok {
		res.Severity = envmerge.SeverityError
		res.Code = CodeSecretNotFound
		res.Message = "no stored value for this reference"
		return res
	}

	algorithm, payload, err := envelope.Decode(ciphertext)
	if err != nil {
		res.Severity = envmerge.SeverityError
		res.Code = CodeNotEncrypted
		res.Message = "the stored value is not encrypted"
		return res
	}
	if algorithm != r.cipher.Algorithm() {
		res.Severity = envmerge.SeverityError
		res.Code = CodeAlgorithmMismatch
		res.Message = fmt.Sprintf(
			"stored with %q, but the configured cipher is %q",
			algorithm, r.cipher.Algorithm(),
		)
		return res
	}

	privateKey, err := r.groupPrivateKey(ref.group)
	if err != nil {
		return referenceKeyResolution(res, err)
	}

	// Decrypt to establish status. The plaintext is retained only when the
	// resolver reveals; otherwise it is discarded when this function returns.
	plaintext, err := r.cipher.Decrypt(payload, privateKey)
	if err != nil {
		res.Severity = envmerge.SeverityError
		res.Code = CodeInvalidPrivateKey
		res.Message = "the private key for this group does not decrypt the value"
		return res
	}

	res.Severity = envmerge.SeverityOK
	res.Code = CodeOK
	if r.reveal {
		res.Resolved = plaintext
		res.HasResolved = true
	}
	return res
}

// referenceKeyResolution maps a private-key resolution failure onto a status: an
// absent key is a warning, while a present but malformed key is an error.
func referenceKeyResolution(
	res envmerge.Resolution, err error,
) envmerge.Resolution {
	if errors.Is(err, privatekey.ErrNotAvailable) {
		res.Severity = envmerge.SeverityWarning
		res.Code = CodePrivateKeyUnavailable
		res.Message = "no private key for this group in this context"
		return res
	}
	if errors.Is(err, privatekey.ErrInvalidKey) || errors.Is(err, cipher.ErrInvalidKey) {
		res.Severity = envmerge.SeverityError
		res.Code = CodeInvalidPrivateKey
		res.Message = "the private key for this group is malformed"
		return res
	}
	res.Severity = envmerge.SeverityError
	res.Code = CodeInvalidPrivateKey
	res.Message = "the private key for this group could not be resolved"
	return res
}
