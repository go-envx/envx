package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// Encrypt encrypts matching plaintext store entries in place and reports the
// identities it changed. An empty group or key widens the selection to every
// value in that dimension, while a non-empty value narrows it. Values already
// carrying a ciphertext envelope are left untouched, so the operation is
// idempotent. An explicit selector that matches no stored entry is an error.
// Every matching value is re-encrypted in memory before the store is written, so
// a mid-operation failure leaves the store unchanged.
func (m *Manager) Encrypt(group, key string) (UpdateResult, error) {
	// Normalize and validate the selector before touching the store.
	selection, err := newSelection(group, key)
	if err != nil {
		return UpdateResult{}, err
	}

	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return UpdateResult{}, err
	}

	// Encrypt every matching plaintext value into staged changes first, so a
	// failure partway through never writes a partially encrypted store.
	var changes []store.Secret
	var references []SecretReference
	matched := false
	for _, secret := range document.Secrets() {
		if !selection.matches(secret) {
			continue
		}
		matched = true
		if envelope.IsCiphertext(secret.Value) {
			continue
		}

		ciphertext, err := m.encryptStoredValue(document, secret)
		if err != nil {
			return UpdateResult{}, err
		}
		changes = append(changes, store.Secret{
			Group: secret.Group, Key: secret.Key, Value: ciphertext,
		})
		references = append(references, SecretReference{
			Group: secret.Group, Key: secret.Key,
		})
	}

	if selection.explicit && !matched {
		return UpdateResult{}, selection.noMatchError()
	}
	return m.applyBulkChanges(document, changes, references)
}

// Decrypt decrypts matching ciphertext store entries in place and reports the
// identities it changed. An empty group or key widens the selection to every
// value in that dimension, while a non-empty value narrows it. Values already
// stored as plaintext are left untouched, so the operation is idempotent. Each
// group's private key is resolved lazily and cached, so only groups that carry a
// matching ciphertext require a key. A group whose private key is unavailable is
// skipped and reported through UpdateResult.Unavailable rather than failing the
// whole operation, so an available key still decrypts its own group. An explicit
// selector that matches no stored entry is an error. Every decryptable value is
// staged in memory before the store is written, so a mid-operation failure leaves
// the store unchanged.
func (m *Manager) Decrypt(group, key string) (UpdateResult, error) {
	// Normalize and validate the selector before touching the store.
	selection, err := newSelection(group, key)
	if err != nil {
		return UpdateResult{}, err
	}

	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return UpdateResult{}, err
	}

	// Decrypt every matching ciphertext value into staged changes first, so a
	// failure partway through never writes a partially decrypted store.
	keys := newPrivateKeyCache(m.params.PrivateKeyResolver)
	var changes []store.Secret
	var references []SecretReference
	var unavailable []string
	seenUnavailable := make(map[string]struct{})
	matched := false
	for _, secret := range document.Secrets() {
		if !selection.matches(secret) {
			continue
		}
		matched = true
		if !envelope.IsCiphertext(secret.Value) {
			continue
		}

		privateKey, available, err := keys.resolve(secret.Group)
		if err != nil {
			return UpdateResult{}, err
		}
		if !available {
			if _, seen := seenUnavailable[secret.Group]; !seen {
				seenUnavailable[secret.Group] = struct{}{}
				unavailable = append(unavailable, secret.Group)
			}
			continue
		}

		plaintext, err := m.decryptStoredValue(secret, privateKey)
		if err != nil {
			return UpdateResult{}, err
		}
		changes = append(changes, store.Secret{
			Group: secret.Group, Key: secret.Key, Value: plaintext,
		})
		references = append(references, SecretReference{
			Group: secret.Group, Key: secret.Key,
		})
	}

	if selection.explicit && !matched {
		return UpdateResult{}, selection.noMatchError()
	}

	result, err := m.applyBulkChanges(document, changes, references)
	if err != nil {
		return UpdateResult{}, err
	}
	result.Unavailable = unavailable
	return result, nil
}

// encryptStoredValue encrypts one plaintext store entry to its group's public
// key and wraps the result in the algorithm-tagged envelope.
func (m *Manager) encryptStoredValue(
	document *store.Document, secret store.Secret,
) (string, error) {
	publicKey, exists := document.PublicKey(secret.Group)
	if !exists {
		return "", fmt.Errorf(
			"group %q has no public key; run 'envx keypair generate %s' first",
			secret.Group, secret.Group,
		)
	}
	native, err := m.params.Cipher.Encrypt(secret.Value, publicKey)
	if err != nil {
		return "", fmt.Errorf(
			"encrypting secret %q in group %q: %w", secret.Key, secret.Group, err,
		)
	}
	ciphertext, err := envelope.Encode(m.params.Cipher.Algorithm(), native)
	if err != nil {
		return "", fmt.Errorf(
			"encoding secret %q in group %q: %w", secret.Key, secret.Group, err,
		)
	}
	return ciphertext, nil
}

// decryptStoredValue decodes one ciphertext store entry and decrypts it with the
// supplied private key.
func (m *Manager) decryptStoredValue(
	secret store.Secret, privateKey string,
) (string, error) {
	algorithm, payload, err := envelope.Decode(secret.Value)
	if err != nil {
		return "", fmt.Errorf(
			"secret %q in group %q is not encrypted: %w",
			secret.Key, secret.Group, err,
		)
	}
	if algorithm != m.params.Cipher.Algorithm() {
		return "", fmt.Errorf(
			"secret %q in group %q uses algorithm %q, but the configured cipher is %q",
			secret.Key, secret.Group, algorithm, m.params.Cipher.Algorithm(),
		)
	}
	plaintext, err := m.params.Cipher.Decrypt(payload, privateKey)
	if err != nil {
		return "", fmt.Errorf(
			"decrypting secret %q in group %q: %w", secret.Key, secret.Group, err,
		)
	}
	return plaintext, nil
}

// privateKeyCache resolves each group's private key once, distinguishing an
// unavailable key (a reportable condition) from a hard resolution error.
type privateKeyCache struct {
	// resolver supplies a group's private-key material.
	resolver privatekey.Resolver
	// keys caches resolved private keys by group.
	keys map[string]string
	// missing records groups already known to have no available key.
	missing map[string]struct{}
}

// newPrivateKeyCache creates an empty per-group private-key cache.
func newPrivateKeyCache(resolver privatekey.Resolver) *privateKeyCache {
	return &privateKeyCache{
		resolver: resolver,
		keys:     make(map[string]string),
		missing:  make(map[string]struct{}),
	}
}

// resolve returns a group's private key, reporting available=false when no
// source has a key for the group. Any other resolution failure is an error.
func (c *privateKeyCache) resolve(
	group string,
) (privateKey string, available bool, err error) {
	if key, ok := c.keys[group]; ok {
		return key, true, nil
	}
	if _, ok := c.missing[group]; ok {
		return "", false, nil
	}
	resolved, err := c.resolver.Resolve(group)
	if err != nil {
		if errors.Is(err, privatekey.ErrNotAvailable) {
			c.missing[group] = struct{}{}
			return "", false, nil
		}
		return "", false, fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}
	c.keys[group] = resolved.Value
	return resolved.Value, true, nil
}

// applyBulkChanges writes the staged value changes atomically and returns the
// changed identities. It performs no write when nothing changed.
func (m *Manager) applyBulkChanges(
	document *store.Document,
	changes []store.Secret,
	references []SecretReference,
) (UpdateResult, error) {
	if len(changes) == 0 {
		return UpdateResult{}, nil
	}
	for _, change := range changes {
		if err := document.SetSecret(
			change.Group, change.Key, change.Value,
		); err != nil {
			return UpdateResult{}, err
		}
	}
	if err := document.Save(m.params.DefaultIndent); err != nil {
		return UpdateResult{}, fmt.Errorf("saving secrets store: %w", err)
	}
	return UpdateResult{Secrets: references}, nil
}

// selection is a normalized, validated bulk selector over stored secrets. An
// empty group or key matches every value in that dimension.
type selection struct {
	// group is the normalized group filter; empty matches all groups.
	group string
	// key is the exact key filter; empty matches all keys.
	key string
	// explicit reports whether the caller narrowed the selection, so a selection
	// that matches nothing can be rejected only when it was deliberate.
	explicit bool
}

// newSelection normalizes and validates a bulk selector. Empty dimensions are
// left unvalidated because they intentionally match everything.
func newSelection(group, key string) (selection, error) {
	sel := selection{explicit: group != "" || key != ""}
	if group != "" {
		normalized, err := normalizeGroupName(group)
		if err != nil {
			return selection{}, err
		}
		sel.group = normalized
	}
	if key != "" {
		if err := validateSecretKey(key); err != nil {
			return selection{}, err
		}
		sel.key = key
	}
	return sel, nil
}

// matches reports whether a stored secret falls within the selection. Groups are
// matched case-insensitively while keys must match exactly.
func (s selection) matches(secret store.Secret) bool {
	if s.group != "" && !strings.EqualFold(secret.Group, s.group) {
		return false
	}
	if s.key != "" && secret.Key != s.key {
		return false
	}
	return true
}

// noMatchError describes an explicit selector that matched no stored entry.
func (s selection) noMatchError() error {
	switch {
	case s.group != "" && s.key != "":
		return fmt.Errorf("no secret %q found in group %q", s.key, s.group)
	case s.group != "":
		return fmt.Errorf("no secrets found in group %q", s.group)
	default:
		return fmt.Errorf("no secret %q found in any group", s.key)
	}
}
