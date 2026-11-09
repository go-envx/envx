package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/value"
)

// Resolver dereferences "secret://group/key" values for one operation. Other
// values pass through unchanged. A masking resolver never decrypts; group
// snapshots and private keys are cached only for the resolver's lifetime.
type Resolver struct {
	// reader reads group snapshots on first use.
	reader Reader
	// keys resolves private keys on first use per group.
	keys KeySource
	// cipher decrypts revealed values.
	cipher Decrypter
	// reveal decrypts references instead of masking them.
	reveal bool
	// groups caches snapshots read during this operation.
	groups map[string]Snapshot
	// privateKeys caches key resolutions, including failures, by group.
	privateKeys map[string]keyLookup
}

// keyLookup caches a private-key resolution, including its failure.
type keyLookup struct {
	// value is the resolved private key.
	value string
	// err is the resolution failure, if any.
	err error
}

// NewResolver constructs an operation-scoped resolver.
func NewResolver(
	reader Reader, keys KeySource, cipher Decrypter, reveal bool,
) (*Resolver, error) {
	if reader == nil {
		return nil, errors.New("secrets reader is required")
	}
	if keys == nil {
		return nil, errors.New("private keys are required")
	}
	if cipher == nil {
		return nil, errors.New("cipher is required")
	}
	return &Resolver{
		reader:      reader,
		keys:        keys,
		cipher:      cipher,
		reveal:      reveal,
		groups:      make(map[string]Snapshot),
		privateKeys: make(map[string]keyLookup),
	}, nil
}

// Resolve returns a value with any secret reference masked to its canonical
// form or, when revealing, decrypted. A leading backslash escapes a literal
// reference. Malformed and dangling revealed references are errors.
func (r *Resolver) Resolve(raw, environment string) (string, error) {
	if literal, ok := strings.CutPrefix(raw, `\`+scheme); ok {
		return scheme + literal, nil
	}
	ref, err := ParseReference(raw, environment)
	if errors.Is(err, ErrNotReference) {
		return raw, nil
	}
	if err != nil {
		return "", err
	}
	if !r.reveal {
		return ref.String(), nil
	}

	snap, err := r.group(ref.Group)
	if err != nil {
		return "", err
	}
	stored, ok := snap.Group.Values[ref.Key]
	if !ok {
		return "", notFound(ref)
	}
	payload, err := openEnvelope(ref, stored, r.cipher.Algorithm())
	if err != nil {
		return "", err
	}
	key, err := r.privateKey(ref.Group, snap.Group.PublicKey)
	if err != nil {
		return "", fmt.Errorf("resolving private key for group %q: %w", ref.Group, err)
	}
	plaintext, err := r.cipher.Decrypt(payload, key)
	if err != nil {
		return "", fmt.Errorf(
			"decrypting secret %q in group %q: %w", ref.Key, ref.Group, err,
		)
	}
	return plaintext, nil
}

// Evaluate classifies a value and diagnoses secret references without
// returning an error. Masking evaluation inspects records but never decrypts,
// and findings never contain private keys or plaintext.
func (r *Resolver) Evaluate(raw, environment string) value.Evaluation {
	if literal, ok := strings.CutPrefix(raw, `\`+scheme); ok {
		return value.Evaluation{Kind: value.Config, Value: scheme + literal, Resolved: true}
	}
	ref, err := ParseReference(raw, environment)
	if errors.Is(err, ErrNotReference) {
		return value.Evaluation{Kind: value.Config, Value: raw, Resolved: true}
	}
	if err != nil {
		return value.Evaluation{
			Kind:       value.Secret,
			Value:      raw,
			Diagnostic: r.finding(diagnostic.InvalidSecretReference, err.Error()),
		}
	}

	eval := value.Evaluation{Kind: value.Secret, Value: ref.String()}
	payload, publicKey, found := r.inspect(ref)
	if !found.IsZero() {
		eval.Diagnostic = found
		return eval
	}
	key, err := r.privateKey(ref.Group, publicKey)
	if err != nil {
		eval.Diagnostic = r.keyFinding(err)
		return eval
	}
	if !r.reveal {
		return eval
	}
	plaintext, err := r.cipher.Decrypt(payload, key)
	if err != nil {
		eval.Diagnostic = r.finding(
			diagnostic.PrivateKeyIsInvalid,
			"the private key for this group does not decrypt the value",
		)
		return eval
	}
	eval.Value = plaintext
	eval.Resolved = true
	return eval
}

// inspect locates a reference's envelope, reporting a finding for a record
// that cannot be decrypted with the configured cipher.
func (r *Resolver) inspect(
	ref Reference,
) (payload []byte, publicKey string, found diagnostic.Finding) {
	snap, err := r.group(ref.Group)
	if err != nil {
		return nil, "", r.finding(
			diagnostic.SecretReferenceIsUnresolved, "the secrets store could not be read",
		)
	}
	stored, ok := snap.Group.Values[ref.Key]
	if !ok {
		return nil, "", r.finding(
			diagnostic.SecretReferenceNotFound, "no stored value for this reference",
		)
	}
	env, err := decodeEnvelope(stored)
	if err != nil {
		return nil, "", r.finding(
			diagnostic.SecretIsNotEncrypted, "the stored value is not encrypted",
		)
	}
	if configured := r.cipher.Algorithm(); !env.sealedBy(configured) {
		return nil, "", r.finding(diagnostic.SecretAlgorithmMismatch, fmt.Sprintf(
			"stored with %q, but the configured cipher is %q", env.algorithm, configured,
		))
	}
	return env.payload, snap.Group.PublicKey, diagnostic.Finding{}
}

// group reads and caches one group snapshot.
func (r *Resolver) group(name string) (Snapshot, error) {
	if snap, ok := r.groups[name]; ok {
		return snap, nil
	}
	snap, err := r.reader.Read(name)
	if err != nil {
		return Snapshot{}, err
	}
	r.groups[name] = snap
	return snap, nil
}

// privateKey resolves and caches one group's private key or failure.
func (r *Resolver) privateKey(group, publicKey string) (string, error) {
	lookup, ok := r.privateKeys[group]
	if !ok {
		key, err := r.keys.Resolve(group, publicKey)
		lookup = keyLookup{value: key.Value, err: err}
		r.privateKeys[group] = lookup
	}
	return lookup.value, lookup.err
}

// keyFinding maps a private-key failure onto a finding: an absent key warns,
// while a malformed or unresolvable key is an error.
func (r *Resolver) keyFinding(err error) diagnostic.Finding {
	switch {
	case errors.Is(err, ErrKeyNotAvailable):
		return r.finding(
			diagnostic.PrivateKeyIsUnavailable,
			"no private key for this group in this context",
		)
	case errors.Is(err, ErrInvalidKey):
		return r.finding(
			diagnostic.PrivateKeyIsInvalid, "the private key for this group is malformed",
		)
	default:
		return r.finding(
			diagnostic.PrivateKeyIsInvalid,
			"the private key for this group could not be resolved",
		)
	}
}

// finding builds a finding at the code's default severity.
func (r *Resolver) finding(code diagnostic.Code, message string) diagnostic.Finding {
	def, _ := diagnostic.Lookup(code)
	return diagnostic.Finding{Code: code, Level: def.Level, Message: message}
}
