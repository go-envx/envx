package secrets

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// PlaintextResolver lazily supplies one secret plaintext value.
type PlaintextResolver func() (string, error)

// Set obtains one plaintext value lazily, encrypts it, and stores its
// algorithm-tagged ciphertext. The group must already have a public key;
// keypair creation is a separate operation so this method cannot create
// partially configured identities. The plaintext source is not called until
// group and key validation and public-key lookup have succeeded.
func (m *Manager) Set(group, key string, plaintextSource PlaintextResolver) error {
	// Normalize names and reject invalid input before loading the store.
	group, err := normalizeGroupName(group)
	if err != nil {
		return err
	}
	if err := validateSecretKey(key); err != nil {
		return err
	}
	if plaintextSource == nil {
		return errors.New("plaintext source is nil")
	}

	// Require the group's public key before requesting plaintext.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return err
	}
	publicKey, exists := document.PublicKey(group)
	if !exists {
		return fmt.Errorf(
			"group %q has no public key; run 'envx keypair generate %s' first",
			group, group,
		)
	}

	// Resolve plaintext only after all validation and lookup have succeeded.
	plaintext, err := plaintextSource()
	if err != nil {
		return err
	}
	if plaintext == "" {
		return errors.New("secret plaintext is empty")
	}

	// Encrypt the value and tag it with the configured algorithm.
	nativeCiphertext, err := m.params.Cipher.Encrypt(plaintext, publicKey)
	if err != nil {
		return fmt.Errorf("encrypting secret %q in group %q: %w", key, group, err)
	}
	ciphertext, err := envelope.Encode(
		m.params.Cipher.Algorithm(), nativeCiphertext,
	)
	if err != nil {
		return fmt.Errorf("encoding secret %q in group %q: %w", key, group, err)
	}

	// Store the tagged ciphertext and persist the updated document.
	if err := document.SetSecret(group, key, ciphertext); err != nil {
		return err
	}
	if err := document.Save(m.params.DefaultIndent); err != nil {
		return fmt.Errorf("saving secret %q in group %q: %w", key, group, err)
	}
	return nil
}

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

// Delete removes one stored secret value and persists the store. The group's
// public key and its remaining values are preserved, since tearing down a group
// identity has its own retention semantics and is a separate operation. A
// missing entry is a dangling reference and an error.
func (m *Manager) Delete(group, key string) error {
	// Normalize names and reject invalid input before loading the store.
	group, err := normalizeGroupName(group)
	if err != nil {
		return err
	}
	if err := validateSecretKey(key); err != nil {
		return err
	}

	// Remove the located value, failing when the entry is absent.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return err
	}
	deleted, err := document.DeleteSecret(group, key)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("secret %q not found in group %q", key, group)
	}

	// Persist the store only after the removal has been applied in memory.
	if err := document.Save(m.params.DefaultIndent); err != nil {
		return fmt.Errorf(
			"saving deletion of secret %q in group %q: %w", key, group, err,
		)
	}
	return nil
}
