package secrets

import (
	"errors"
	"fmt"
)

// SetSecret obtains one plaintext value lazily, encrypts it, and stores its
// algorithm-tagged ciphertext. The group must already have a public key;
// keypair creation is a separate operation so this method cannot create
// partially configured identities. The plaintext source is not called until
// group and key validation and public-key lookup have succeeded.
func (s *Service) SetSecret(
	group, key string, plaintextSource PlaintextResolver,
) (SetSecretResult, error) {
	// Normalize names and reject invalid input before querying the repository.
	var err error
	group, err = NormalizeGroupName(group)
	if err != nil {
		return SetSecretResult{}, err
	}
	if err := ValidateSecretKey(key); err != nil {
		return SetSecretResult{}, err
	}
	if plaintextSource == nil {
		return SetSecretResult{}, errors.New("plaintext source is nil")
	}

	// Require the group's public key before requesting plaintext.
	publicKey, exists, err := s.params.Repository.GetPublicKey(group)
	if err != nil {
		return SetSecretResult{}, err
	}
	if !exists {
		return SetSecretResult{}, fmt.Errorf(
			"group %q has no public key; run 'envx keypair generate %s' first",
			group, group,
		)
	}

	// Resolve plaintext only after all validation and lookup have succeeded.
	plaintext, err := plaintextSource()
	if err != nil {
		return SetSecretResult{}, err
	}
	if plaintext == "" {
		return SetSecretResult{}, errors.New("secret plaintext is empty")
	}

	// Encrypt the value and tag it with the configured algorithm.
	nativeCiphertext, err := s.params.Cipher.Encrypt(plaintext, publicKey)
	if err != nil {
		return SetSecretResult{}, fmt.Errorf(
			"encrypting secret %q in group %q: %w", key, group, err,
		)
	}
	ciphertext, err := s.codec.encode(
		s.params.Cipher.Algorithm(), nativeCiphertext,
	)
	if err != nil {
		return SetSecretResult{}, fmt.Errorf(
			"encoding secret %q in group %q: %w", key, group, err,
		)
	}

	// Store the tagged ciphertext via the repository.
	record := SecretRecord{
		Group:      group,
		Key:        key,
		Ciphertext: ciphertext,
	}
	if err := s.params.Repository.SetSecret(record); err != nil {
		return SetSecretResult{}, fmt.Errorf(
			"saving secret %q in group %q: %w", key, group, err,
		)
	}
	return SetSecretResult{
		Location: s.Location(),
		Secret: SecretReference{
			Group: group,
			Key:   key,
		},
	}, nil
}

// GetSecret decrypts and returns one stored secret value and its location. The
// group must exist and the key must be present; a missing entry is a dangling
// reference and an error. The private key is resolved only after the ciphertext
// has been located, and an unavailable key fails the operation because GetSecret
// promises plaintext.
func (s *Service) GetSecret(group, key string) (GetSecretResult, error) {
	// Normalize names and reject invalid input before loading the store.
	var err error
	group, err = NormalizeGroupName(group)
	if err != nil {
		return GetSecretResult{}, err
	}
	if err := ValidateSecretKey(key); err != nil {
		return GetSecretResult{}, err
	}

	// Locate the stored ciphertext for the requested identity.
	secret, exists, err := s.params.Repository.GetSecret(group, key)
	if err != nil {
		return GetSecretResult{}, err
	}
	if !exists {
		return GetSecretResult{}, fmt.Errorf("secret %q not found in group %q", key, group)
	}

	// Decode the algorithm-tagged envelope before resolving any private key.
	algorithm, payload, err := s.codec.decode(secret.Ciphertext)
	if err != nil {
		return GetSecretResult{}, fmt.Errorf(
			"secret %q in group %q is not encrypted: %w", key, group, err,
		)
	}
	if algorithm != s.params.Cipher.Algorithm() {
		return GetSecretResult{}, fmt.Errorf(
			"secret %q in group %q uses algorithm %q, but the configured cipher is %q",
			key, group, algorithm, s.params.Cipher.Algorithm(),
		)
	}

	// Resolve the private key only after the ciphertext has been located.
	privateKey, err := s.params.PrivateKeyService.Resolve(group)
	if err != nil {
		return GetSecretResult{}, fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}

	// Decrypt the located ciphertext into transient plaintext.
	plaintext, err := s.params.Cipher.Decrypt(payload, privateKey.Value)
	if err != nil {
		return GetSecretResult{}, fmt.Errorf(
			"decrypting secret %q in group %q: %w", key, group, err,
		)
	}
	return GetSecretResult{
		Location: s.Location(),
		Value:    plaintext,
	}, nil
}

// Has reports whether one secret entry exists without loading a private key or
// decrypting its value.
func (s *Service) Has(group, key string) (bool, error) {
	// Normalize names and reject invalid input before checking repository.
	var err error
	group, err = NormalizeGroupName(group)
	if err != nil {
		return false, err
	}
	if err := ValidateSecretKey(key); err != nil {
		return false, err
	}

	// Report presence directly from the repository without touching key material.
	_, exists, err := s.params.Repository.GetSecret(group, key)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// DeleteSecret removes one stored secret value and persists the store. The group's
// public key and its remaining values are preserved, since tearing down a group
// identity has its own retention semantics and is a separate operation. A
// missing entry is a dangling reference and an error.
func (s *Service) DeleteSecret(group, key string) (DeleteSecretResult, error) {
	// Normalize names and reject invalid input before querying repository.
	var err error
	group, err = NormalizeGroupName(group)
	if err != nil {
		return DeleteSecretResult{}, err
	}
	if err := ValidateSecretKey(key); err != nil {
		return DeleteSecretResult{}, err
	}

	// Remove the located value, failing when the entry is absent.
	_, exists, err := s.params.Repository.GetSecret(group, key)
	if err != nil {
		return DeleteSecretResult{}, err
	}
	if !exists {
		return DeleteSecretResult{}, fmt.Errorf("secret %q not found in group %q", key, group)
	}

	if err := s.params.Repository.DeleteSecret(group, key); err != nil {
		return DeleteSecretResult{}, err
	}

	return DeleteSecretResult{
		Location: s.Location(),
		Secret: SecretReference{
			Group: group,
			Key:   key,
		},
	}, nil
}
