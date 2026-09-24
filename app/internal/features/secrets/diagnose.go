package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/features/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/status"
	"github.com/go-envx/envx/app/internal/shared/value"
	"github.com/go-envx/envx/app/internal/utils/severity"
)

// ErrSecretNotFound indicates a reference to a value absent from the store. It
// is the dangling-reference sentinel, distinct from the cipher and private-key
// sentinels a diagnosis reuses for the remaining failure modes.
var ErrSecretNotFound = errors.New("secret not found")

var (
	_ value.Resolver  = (*Resolver)(nil)
	_ value.Evaluator = (*Resolver)(nil)
)

// Evaluate implements value.Evaluator, reporting a value's kind and dry-run
// resolution without materializing plaintext unless the resolver reveals. It
// never returns an error; a failure is reported through the Evaluation's
// severity and status code, and no private-key or resolved-secret material
// appears in Status or StatusMessage.
func (r *Resolver) Evaluate(val, _ string) value.Evaluation {
	// Unescape a literal that starts with the reserved scheme by dropping only the
	// leading backslash, matching Resolve so its literal is "secret://...".
	if strings.HasPrefix(val, `\`+scheme) {
		return r.configValueResolution(val[1:])
	}
	// Ordinary values are plain configuration.
	if !strings.HasPrefix(val, scheme) {
		return r.configValueResolution(val)
	}

	// Parse the reference; malformed grammar is an error regardless of policy.
	body := strings.TrimPrefix(val, scheme)
	ref, err := splitRef(body)
	if err != nil {
		return value.Resolution{
			Kind:     value.KindSecret,
			Severity: severity.Error,
			Status:   status.InvalidSecretReference,
			Code:     status.InvalidSecretReference,
			Message:  err.Error(),
		}
	}
	return r.diagnoseReference(ref)
}

// Diagnose preserves backwards compatibility during migration, delegating to Evaluate.
func (r *Resolver) Diagnose(val, env string) value.Evaluation {
	return r.Evaluate(val, env)
}

// configValueResolution reports an ok config value, attaching its resolved form
// only when the resolver reveals.
func (r *Resolver) configValueResolution(val string) value.Evaluation {
	res := value.Evaluation{
		Kind:     value.KindConfig,
		Severity: severity.OK,
		Status:   status.OK,
		Code:     status.OK,
	}
	if r.reveal {
		res.Value = val
		res.Resolved = val
		res.IsResolved = true
		res.HasResolved = true
	}
	return res
}

// diagnoseReference classifies a well-formed reference by attempting decryption
// for status only. It discards any plaintext unless the resolver reveals, and
// maps typed cipher and private-key failures onto stable status codes.
func (r *Resolver) diagnoseReference(ref reference) value.Evaluation {
	res := value.Evaluation{Kind: value.KindSecret}

	ciphertext, ok := r.values[ref]
	if !ok {
		res.Severity = severity.Error
		res.Status = status.SecretReferenceNotFound
		res.Code = status.SecretReferenceNotFound
		res.StatusMessage = "no stored value for this reference"
		res.Message = "no stored value for this reference"
		return res
	}

	algorithm, payload, err := envelope.Decode(ciphertext)
	if err != nil {
		res.Severity = severity.Error
		res.Status = status.SecretIsNotEncrypted
		res.Code = status.SecretIsNotEncrypted
		res.StatusMessage = "the stored value is not encrypted"
		res.Message = "the stored value is not encrypted"
		return res
	}
	if algorithm != r.cipher.Algorithm() {
		res.Severity = severity.Error
		res.Status = status.SecretAlgorithmMismatch
		res.Code = status.SecretAlgorithmMismatch
		res.StatusMessage = fmt.Sprintf(
			"stored with %q, but the configured cipher is %q",
			algorithm, r.cipher.Algorithm(),
		)
		res.Message = res.StatusMessage
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
		res.Severity = severity.Error
		res.Status = status.PrivateKeyIsInvalid
		res.Code = status.PrivateKeyIsInvalid
		res.StatusMessage = "the private key for this group does not decrypt the value"
		res.Message = res.StatusMessage
		return res
	}

	res.Severity = severity.OK
	res.Status = status.OK
	res.Code = status.OK
	if r.reveal {
		res.Value = plaintext
		res.Resolved = plaintext
		res.IsResolved = true
		res.HasResolved = true
	}
	return res
}

// referenceKeyResolution maps a private-key resolution failure onto a status: an
// absent key is a warning, while a present but malformed key is an error.
func referenceKeyResolution(
	res value.Evaluation, err error,
) value.Evaluation {
	if errors.Is(err, privatekey.ErrNotAvailable) {
		res.Severity = severity.Warn
		res.Status = status.PrivateKeyIsUnavailable
		res.Code = status.PrivateKeyIsUnavailable
		res.StatusMessage = "no private key for this group in this context"
		res.Message = res.StatusMessage
		return res
	}
	if errors.Is(err, privatekey.ErrInvalidKey) || errors.Is(err, cipher.ErrInvalidKey) {
		res.Severity = severity.Error
		res.Status = status.PrivateKeyIsInvalid
		res.Code = status.PrivateKeyIsInvalid
		res.StatusMessage = "the private key for this group is malformed"
		res.Message = res.StatusMessage
		return res
	}
	res.Severity = severity.Error
	res.Status = status.PrivateKeyIsInvalid
	res.Code = status.PrivateKeyIsInvalid
	res.StatusMessage = "the private key for this group could not be resolved"
	res.Message = res.StatusMessage
	return res
}
