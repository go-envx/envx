package secrets

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// -------------------------------------------------------------------------------------

// PlaintextResolver lazily supplies one secret plaintext value.
type PlaintextResolver func() (string, error)

// -------------------------------------------------------------------------------------

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
			"group %q has no public key; run 'envx secrets keypair generate %s' first",
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
	if err := document.Save(); err != nil {
		return fmt.Errorf("saving secret %q in group %q: %w", key, group, err)
	}
	return nil
}
