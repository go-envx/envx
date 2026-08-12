package secrets

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// Get decrypts and returns one stored secret value. The group must exist and the
// key must be present; a missing entry is a dangling reference and an error. The
// private key is resolved only after the ciphertext has been located, and an
// unavailable key fails the operation because Get promises plaintext.
func (m *Manager) Get(group, key string) (string, error) {
	// Normalize names and reject invalid input before loading the store.
	group, err := normalizeGroupName(group)
	if err != nil {
		return "", err
	}
	if err := validateSecretKey(key); err != nil {
		return "", err
	}

	// Locate the stored ciphertext for the requested identity.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return "", err
	}
	secret, exists := document.Secret(group, key)
	if !exists {
		return "", fmt.Errorf("secret %q not found in group %q", key, group)
	}

	// Decode the algorithm-tagged envelope before resolving any private key.
	algorithm, payload, err := envelope.Decode(secret.Value)
	if err != nil {
		return "", fmt.Errorf(
			"secret %q in group %q is not encrypted: %w", key, group, err,
		)
	}
	if algorithm != m.params.Cipher.Algorithm() {
		return "", fmt.Errorf(
			"secret %q in group %q uses algorithm %q, but the configured cipher is %q",
			key, group, algorithm, m.params.Cipher.Algorithm(),
		)
	}

	// Resolve the private key only after the ciphertext has been located.
	privateKey, err := m.params.PrivateKeyResolver.Resolve(group)
	if err != nil {
		return "", fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}

	// Decrypt the located ciphertext into transient plaintext.
	plaintext, err := m.params.Cipher.Decrypt(payload, privateKey.Value)
	if err != nil {
		return "", fmt.Errorf(
			"decrypting secret %q in group %q: %w", key, group, err,
		)
	}
	return plaintext, nil
}

// Has reports whether one secret entry exists without loading a private key or
// decrypting its value.
func (m *Manager) Has(group, key string) (bool, error) {
	// Normalize names and reject invalid input before loading the store.
	group, err := normalizeGroupName(group)
	if err != nil {
		return false, err
	}
	if err := validateSecretKey(key); err != nil {
		return false, err
	}

	// Report presence directly from the store without touching key material.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return false, err
	}
	_, exists := document.Secret(group, key)
	return exists, nil
}
