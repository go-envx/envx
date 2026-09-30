package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/privatekey"
)

// EncryptSecrets encrypts matching plaintext store entries in place and reports the
// identities it changed. An empty group or key widens the selection to every
// value in that dimension, while a non-empty value narrows it. Values already
// carrying a ciphertext envelope are left untouched, so the operation is
// idempotent. An explicit selector that matches no stored entry is an error.
// Every matching value is re-encrypted in memory before the store is written, so
// a mid-operation failure leaves the store unchanged.
func (s *Service) EncryptSecrets(group, key string) (EncryptSecretsResult, error) {
	// Normalize and validate the selector before touching the store.
	selection, err := newSelection(group, key)
	if err != nil {
		return EncryptSecretsResult{}, err
	}

	allSecrets, err := s.params.Repository.ListSecrets()
	if err != nil {
		return EncryptSecretsResult{}, err
	}

	// Encrypt every matching plaintext value into staged changes first, so a
	// failure partway through never writes a partially encrypted store.
	var changes []SecretRecord
	var references []SecretReference
	matched := false
	for _, secret := range allSecrets {
		if !selection.matches(secret) {
			continue
		}
		matched = true
		if s.codec.isCiphertext(secret.Ciphertext) {
			continue
		}

		ciphertext, err := s.encryptStoredValue(secret)
		if err != nil {
			return EncryptSecretsResult{}, err
		}
		changes = append(changes, SecretRecord{
			Group:      secret.Group,
			Key:        secret.Key,
			Ciphertext: ciphertext,
		})
		references = append(references, SecretReference{
			Group: secret.Group, Key: secret.Key,
		})
	}

	if selection.explicit && !matched {
		return EncryptSecretsResult{}, selection.noMatchError()
	}
	res, err := s.applyBulkChanges(changes, references)
	if err != nil {
		return EncryptSecretsResult{}, err
	}
	return EncryptSecretsResult(res), nil
}

// DecryptSecrets decrypts matching ciphertext store entries in place and reports the
// identities it changed. An empty group or key widens the selection to every
// value in that dimension, while a non-empty value narrows it. Values already
// stored as plaintext are left untouched, so the operation is idempotent. Each
// group's private key is resolved lazily and cached, so only groups that carry a
// matching ciphertext require a key. A group whose private key is unavailable is
// skipped and reported through DecryptSecretsResult.UnavailableGroups rather
// than failing the whole operation, so an available key still decrypts its own
// group. An explicit selector that matches no stored entry is an error. Every
// decryptable value is staged in memory before the store is written, so a
// mid-operation failure leaves the store unchanged.
func (s *Service) DecryptSecrets(group, key string) (DecryptSecretsResult, error) {
	// Normalize and validate the selector before touching the store.
	selection, err := newSelection(group, key)
	if err != nil {
		return DecryptSecretsResult{}, err
	}

	allSecrets, err := s.params.Repository.ListSecrets()
	if err != nil {
		return DecryptSecretsResult{}, err
	}

	// Decrypt every matching ciphertext value into staged changes first, so a
	// failure partway through never writes a partially decrypted store.
	keys := newPrivateKeyCache(s.params.PrivateKeyService)
	var changes []SecretRecord
	var references []SecretReference
	var unavailable []string
	seenUnavailable := make(map[string]struct{})
	matched := false
	for _, secret := range allSecrets {
		if !selection.matches(secret) {
			continue
		}
		matched = true
		if !s.codec.isCiphertext(secret.Ciphertext) {
			continue
		}

		privateKey, available, err := keys.resolve(secret.Group)
		if err != nil {
			return DecryptSecretsResult{}, err
		}
		if !available {
			if _, seen := seenUnavailable[secret.Group]; !seen {
				seenUnavailable[secret.Group] = struct{}{}
				unavailable = append(unavailable, secret.Group)
			}
			continue
		}

		plaintext, err := s.decryptStoredValue(secret, privateKey)
		if err != nil {
			return DecryptSecretsResult{}, err
		}
		changes = append(changes, SecretRecord{
			Group:      secret.Group,
			Key:        secret.Key,
			Ciphertext: plaintext,
		})
		references = append(references, SecretReference{
			Group: secret.Group, Key: secret.Key,
		})
	}

	if selection.explicit && !matched {
		return DecryptSecretsResult{}, selection.noMatchError()
	}

	res, err := s.applyBulkChanges(changes, references)
	if err != nil {
		return DecryptSecretsResult{}, err
	}
	return DecryptSecretsResult{
		Location:          res.Location,
		Secrets:           res.Secrets,
		UnavailableGroups: unavailable,
	}, nil
}

// encryptStoredValue encrypts one plaintext store entry to its group's public
// key and wraps the result in the algorithm-tagged envelope.
func (s *Service) encryptStoredValue(
	secret SecretRecord,
) (string, error) {
	publicKey, exists, err := s.params.Repository.GetPublicKey(secret.Group)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf(
			"group %q has no public key; run 'envx keypair generate %s' first: %w",
			secret.Group, secret.Group, ErrGroupNotFound,
		)
	}
	native, err := s.params.Cipher.Encrypt(secret.Ciphertext, publicKey)
	if err != nil {
		return "", fmt.Errorf(
			"encrypting secret %q in group %q: %w", secret.Key, secret.Group, err,
		)
	}
	ciphertext, err := s.codec.encode(s.params.Cipher.Algorithm(), native)
	if err != nil {
		return "", fmt.Errorf(
			"encoding secret %q in group %q: %w", secret.Key, secret.Group, err,
		)
	}
	return ciphertext, nil
}

// decryptStoredValue decodes one ciphertext store entry and decrypts it with the
// supplied private key.
func (s *Service) decryptStoredValue(
	secret SecretRecord, privateKey string,
) (string, error) {
	algorithm, payload, err := s.codec.decode(secret.Ciphertext)
	if err != nil {
		return "", fmt.Errorf(
			"secret %q in group %q is not encrypted: %w",
			secret.Key, secret.Group, err,
		)
	}
	if algorithm != s.params.Cipher.Algorithm() {
		return "", fmt.Errorf(
			"secret %q in group %q uses algorithm %q, but the configured cipher is %q",
			secret.Key, secret.Group, algorithm, s.params.Cipher.Algorithm(),
		)
	}
	plaintext, err := s.params.Cipher.Decrypt(payload, privateKey)
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
	resolver PrivateKeyService
	// keys caches resolved private keys by group.
	keys map[string]string
	// missing records groups already known to have no available key.
	missing map[string]struct{}
}

// newPrivateKeyCache creates an empty per-group private-key cache.
func newPrivateKeyCache(resolver PrivateKeyService) *privateKeyCache {
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

type bulkChangesResult struct {
	Location string
	Secrets  []SecretReference
}

// applyBulkChanges writes the staged value changes atomically and returns the
// changed identities. It performs no write when nothing changed.
func (s *Service) applyBulkChanges(
	changes []SecretRecord,
	references []SecretReference,
) (bulkChangesResult, error) {
	if len(changes) == 0 {
		return bulkChangesResult{Location: s.Location()}, nil
	}
	if batchRepo, ok := s.params.Repository.(BatchRepository); ok {
		if err := batchRepo.SetSecrets(changes); err != nil {
			return bulkChangesResult{}, fmt.Errorf("saving secrets store: %w", err)
		}
	} else {
		for _, change := range changes {
			if err := s.params.Repository.SetSecret(change); err != nil {
				return bulkChangesResult{}, fmt.Errorf("saving secrets store: %w", err)
			}
		}
	}
	return bulkChangesResult{
		Location: s.Location(),
		Secrets:  references,
	}, nil
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
		normalized, err := NormalizeGroupName(group)
		if err != nil {
			return selection{}, err
		}
		sel.group = normalized
	}
	if key != "" {
		if err := ValidateSecretKey(key); err != nil {
			return selection{}, err
		}
		sel.key = key
	}
	return sel, nil
}

// matches reports whether a stored secret falls within the selection. Groups are
// matched case-insensitively while keys must match exactly.
func (s selection) matches(secret SecretRecord) bool {
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
