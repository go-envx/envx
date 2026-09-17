package secrets

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// StoredSecret reports one stored entry's identity and how its stored value is
// encoded. It never carries the value itself, so a caller can audit the store
// without materializing plaintext.
type StoredSecret struct {
	// Group is the stored key-group name in its stored form.
	Group string
	// Key is the entry name within Group.
	Key string
	// Encrypted reports whether the stored value claims the ciphertext envelope
	// format; a false value marks plaintext left in the store.
	Encrypted bool
	// AlgorithmMismatch reports whether an encrypted value's envelope algorithm
	// differs from the configured cipher, so it could never be decrypted here. It
	// is always false for a plaintext value.
	AlgorithmMismatch bool
}

// StoredSecrets returns every stored entry's identity and encryption state in
// document order, reading the store without decrypting or exposing any value. An
// encrypted value's envelope algorithm is compared to the configured cipher so a
// value stored under a different algorithm is surfaced. A missing store yields an
// empty slice rather than an error, so a workspace with no secrets validates
// cleanly.
func (m *Manager) StoredSecrets() ([]StoredSecret, error) {
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return nil, fmt.Errorf("reading secrets %s: %w", m.params.SecretsPath, err)
	}

	configured := m.params.Cipher.Algorithm()
	stored := document.Secrets()
	out := make([]StoredSecret, 0, len(stored))
	for _, secret := range stored {
		entry := StoredSecret{
			Group:     secret.Group,
			Key:       secret.Key,
			Encrypted: envelope.IsCiphertext(secret.Value),
		}
		// Compare a decodable envelope's algorithm to the configured cipher. A value
		// that is not ciphertext, or a malformed envelope, is left for the encryption
		// check; only a well-formed envelope reports an algorithm mismatch.
		if entry.Encrypted {
			if algorithm, _, decodeErr := envelope.Decode(secret.Value); decodeErr == nil {
				entry.AlgorithmMismatch = algorithm != configured
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// GroupsMissingPublicKey returns every group that has stored secrets but no
// stored public key, in first-seen document order. Such a group's values can
// never be decrypted because there is no key to have encrypted them under, so it
// is a store-health problem no single reference reveals. A missing store yields
// an empty slice rather than an error.
func (m *Manager) GroupsMissingPublicKey() ([]string, error) {
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return nil, fmt.Errorf("reading secrets %s: %w", m.params.SecretsPath, err)
	}

	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, secret := range document.Secrets() {
		if seen[secret.Group] {
			continue
		}
		seen[secret.Group] = true
		if _, ok := document.PublicKey(secret.Group); !ok {
			out = append(out, secret.Group)
		}
	}
	return out, nil
}

// Keypairs returns metadata for every group that declares a public key,
// validating each group's available private key without exposing key material.
// A missing store yields an empty slice. It is the store-level counterpart to
// InspectKeypair a workspace-wide validator uses to surface a broken or
// unavailable keypair no single reference reveals.
func (m *Manager) Keypairs() ([]KeypairMetadata, error) {
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return nil, fmt.Errorf("reading secrets %s: %w", m.params.SecretsPath, err)
	}

	groups := document.PublicKeyGroups()
	out := make([]KeypairMetadata, 0, len(groups))
	for _, group := range groups {
		metadata, err := m.InspectKeypair(group)
		if err != nil {
			return nil, err
		}
		out = append(out, metadata)
	}
	return out, nil
}

// ParseReference reports the group and key a value references, and whether the
// value is a well-formed secret reference. A plain value, an escaped literal
// ("\secret://x"), or a malformed reference yields ok=false. The group is
// lowercased to match how the store indexes references, while the key is
// returned verbatim. It never touches the store, so a caller can enumerate the
// references a configuration uses without resolving them.
func ParseReference(value string) (group, key string, ok bool) {
	if !strings.HasPrefix(value, scheme) {
		return "", "", false
	}
	ref, err := splitRef(strings.TrimPrefix(value, scheme))
	if err != nil {
		return "", "", false
	}
	return ref.group, ref.key, true
}
