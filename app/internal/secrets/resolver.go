package secrets

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
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
	cipher cipher.Cipher
	// privateKeys resolves a group's private key on demand.
	privateKeys privatekey.Resolver
	// resolvedKeys caches each group's private key so it is resolved only once.
	resolvedKeys map[string]string
}

// Resolver opens the current secrets store and returns a resolver bound to it
// with the requested materialization policy. A missing store yields an empty
// resolver, so a reference against it fails loudly as a dangling reference rather
// than leaking the raw reference string.
func (m *Manager) Resolver(params ResolverParams) (*Resolver, error) {
	// Open the current store so references resolve against its contents.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return nil, err
	}

	// Index each stored secret by its normalized resolver reference.
	values := make(map[reference]string)
	for _, secret := range document.Secrets() {
		values[reference{
			group: strings.ToLower(secret.Group),
			key:   secret.Key,
		}] = secret.Value
	}

	return &Resolver{
		values:       values,
		reveal:       params.Reveal,
		cipher:       m.params.Cipher,
		privateKeys:  m.params.PrivateKeyResolver,
		resolvedKeys: make(map[string]string),
	}, nil
}

// Resolve dereferences value against the store. A plain value is returned
// unchanged, and a leading backslash escapes a literal that would otherwise look
// like a reference ("\secret://x" resolves to "secret://x"). A well-formed
// reference is masked to its canonical "secret://group/key" form by default,
// which never consults the store, so a masked read never requires the secret to
// exist. A revealing resolver instead decrypts the referenced value and fails on
// a missing entry (a dangling reference), since existence and decryption are
// reveal-time concerns. A malformed reference is always an error.
func (r *Resolver) Resolve(value, _ string) (string, error) {
	// Unescape a literal value that starts with the reserved scheme.
	if strings.HasPrefix(value, `\`+scheme) {
		return value[1:], nil
	}
	// Leave ordinary values unchanged.
	if !strings.HasPrefix(value, scheme) {
		return value, nil
	}

	// Parse the reference; a malformed reference is an error regardless of policy.
	body := strings.TrimPrefix(value, scheme)
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

// decrypt turns one located ciphertext into plaintext, resolving the referenced
// group's private key lazily so only groups actually referenced require a key.
func (r *Resolver) decrypt(ref reference, ciphertext string) (string, error) {
	// Decode the algorithm-tagged envelope before resolving any private key.
	algorithm, payload, err := envelope.Decode(ciphertext)
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
