package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/shared/status"
	"github.com/go-envx/envx/app/internal/shared/value"
	"github.com/go-envx/envx/app/internal/utils/severity"
)

var (
	_ value.Resolver  = (*Resolver)(nil)
	_ value.Evaluator = (*Resolver)(nil)
)

// ResolverParams controls how one resolver materializes secret references. The
// zero value masks references; revealing them decrypts on lookup and therefore
// requires an available private key for each referenced group.
type ResolverParams struct {
	// Reveal decrypts referenced values instead of masking them.
	Reveal bool
}

// Resolver dereferences secret references against a secrets store. It recognizes
// values of the form "secret://<group>/<key>"; every other value passes through
// unchanged. A masking resolver returns the canonical reference without touching
// any private key, while a revealing resolver decrypts each referenced value.
type Resolver struct {
	// values holds algorithm-tagged ciphertext keyed by its resolver reference.
	values map[reference]string
	// reveal decrypts referenced values instead of masking them.
	reveal bool
	// cipher decrypts revealed ciphertext with the group's private key.
	cipher CipherClient
	// privateKeys resolves a group's private key on demand.
	privateKeys PrivateKeyService
	// resolvedKeys caches each group's private key so it is resolved only once.
	resolvedKeys map[string]string
	// codec decodes algorithm-tagged envelopes.
	codec *envelopeCodec
}

// Resolver opens the current secrets store and returns a resolver bound to it
// with the requested materialization policy. A missing store yields an empty
// resolver, so a reference against it fails loudly as a dangling reference rather
// than leaking the raw reference string.
func (s *Service) Resolver(params ResolverParams) (*Resolver, error) {
	records, err := s.params.Repository.ListSecrets()
	if err != nil {
		return nil, err
	}

	// Index each stored secret by its normalized resolver reference.
	values := make(map[reference]string)
	for _, secret := range records {
		values[reference{
			group: strings.ToLower(secret.Group),
			key:   secret.Key,
		}] = secret.Ciphertext
	}

	return &Resolver{
		values:       values,
		reveal:       params.Reveal,
		cipher:       s.params.Cipher,
		privateKeys:  s.params.PrivateKeyService,
		resolvedKeys: make(map[string]string),
		codec:        s.codec,
	}, nil
}

// Evaluate performs a one-off evaluation of a single reference value.
func (s *Service) Evaluate(ref string, reveal bool) (value.Evaluation, error) {
	r, err := s.Resolver(ResolverParams{Reveal: reveal})
	if err != nil {
		return value.Evaluation{}, err
	}
	return r.Evaluate(ref, ""), nil
}

// Diagnose preserves backwards compatibility during migration, delegating to Evaluate.
func (s *Service) Diagnose(ref string, reveal bool) (value.Evaluation, error) {
	return s.Evaluate(ref, reveal)
}

// Resolve dereferences value against the store. A plain value is returned
// unchanged, and a leading backslash escapes a literal that would otherwise look
// like a reference ("\secret://x" resolves to "secret://x"). A well-formed
// reference is masked to its canonical "secret://group/key" form by default,
// which never consults the store, so a masked read never requires the secret to
// exist. A revealing resolver instead decrypts the referenced value and fails on
// a missing entry (a dangling reference), since existence and decryption are
// reveal-time concerns. A malformed reference is always an error.
func (r *Resolver) Resolve(val, _ string) (string, error) {
	// Unescape a literal value that starts with the reserved scheme.
	if strings.HasPrefix(val, `\`+scheme) {
		return val[1:], nil
	}
	// Leave ordinary values unchanged.
	if !strings.HasPrefix(val, scheme) {
		return val, nil
	}

	// Parse the reference; a malformed reference is an error regardless of policy.
	body := strings.TrimPrefix(val, scheme)
	ref, err := splitRef(body)
	if err != nil {
		return "", err
	}

	// Mask by echoing the canonical reference without consulting the store, so a
	// masked read needs neither the secret to exist nor a private key.
	if !r.reveal {
		return scheme + ref.group + "/" + ref.key, nil
	}

	// Reveal materializes plaintext, so the secret must exist and decrypt. A
	// missing entry is a dangling reference and an error.
	ciphertext, ok := r.values[ref]
	if !ok {
		return "", fmt.Errorf(
			"%w: secret %q not found in group %q", ErrSecretNotFound, ref.key, ref.group,
		)
	}
	return r.decrypt(ref, ciphertext)
}

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
//
// Deprecated: Use Evaluate instead.
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

	algorithm, payload, err := r.codec.decode(ciphertext)
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
	if errors.Is(err, privatekey.ErrInvalidKey) {
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

// decrypt turns one located ciphertext into plaintext, resolving the referenced
// group's private key lazily so only groups actually referenced require a key.
func (r *Resolver) decrypt(ref reference, ciphertext string) (string, error) {
	// Decode the algorithm-tagged envelope before resolving any private key.
	algorithm, payload, err := r.codec.decode(ciphertext)
	if err != nil {
		return "", fmt.Errorf(
			"secret %q in group %q is not encrypted: %w", ref.key, ref.group, err,
		)
	}
	if algorithm != r.cipher.Algorithm() {
		return "", fmt.Errorf(
			"secret %q in group %q uses algorithm %q, but the configured cipher is %q",
			ref.key, ref.group, algorithm, r.cipher.Algorithm(),
		)
	}

	// Resolve the group's private key on demand and decrypt the payload.
	privateKey, err := r.groupPrivateKey(ref.group)
	if err != nil {
		return "", err
	}
	plaintext, err := r.cipher.Decrypt(payload, privateKey)
	if err != nil {
		return "", fmt.Errorf(
			"decrypting secret %q in group %q: %w", ref.key, ref.group, err,
		)
	}
	return plaintext, nil
}

// groupPrivateKey resolves and caches a group's private key so repeated
// references to the same group resolve its key only once.
func (r *Resolver) groupPrivateKey(group string) (string, error) {
	if key, ok := r.resolvedKeys[group]; ok {
		return key, nil
	}
	privateKey, err := r.privateKeys.Resolve(group)
	if err != nil {
		return "", fmt.Errorf("resolving private key for group %q: %w", group, err)
	}
	r.resolvedKeys[group] = privateKey.Value
	return privateKey.Value, nil
}
