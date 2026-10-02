package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/privatekey"
)

// GenerateKeypair creates a missing group identity and commits its public key
// only after the private-key destination has accepted the new private key.
func (s *Service) GenerateKeypair(group string) (GenerateKeypairResult, error) {
	// Normalize the group before using it in storage or key paths.
	var err error
	group, err = NormalizeGroupName(group)
	if err != nil {
		return GenerateKeypairResult{}, err
	}

	// Ensure this group does not already exist.
	if _, exists, err := s.params.Repository.GetPublicKey(group); err != nil {
		return GenerateKeypairResult{}, err
	} else if exists {
		return GenerateKeypairResult{}, fmt.Errorf(
			"group %q already has a public key: %w", group, ErrKeypairExists,
		)
	}

	// Generate and validate both key halves before changing the repository.
	pair, err := s.params.Cipher.Keypair()
	if err != nil {
		return GenerateKeypairResult{}, fmt.Errorf(
			"generating keypair for group %q: %w", group, err,
		)
	}
	pubKey, privKey := pair.PublicKey, pair.PrivateKey
	if pubKey == "" || privKey == "" {
		return GenerateKeypairResult{}, errors.New("cipher generated an incomplete keypair")
	}
	if err := s.params.Cipher.ValidateKeypair(pubKey, privKey); err != nil {
		return GenerateKeypairResult{}, fmt.Errorf("generated keypair is invalid: %w", err)
	}

	// Deliver the private key before committing its matching public key.
	if err := s.params.PrivateKeyService.Set(group, privKey); err != nil {
		return GenerateKeypairResult{}, fmt.Errorf(
			"writing private key for group %q: %w", group, err,
		)
	}

	// Commit the public key after the private key was accepted.
	if err := s.params.Repository.SetPublicKey(group, pubKey); err != nil {
		return GenerateKeypairResult{}, fmt.Errorf(
			"private key for group %q was written, but public key could not be committed: %w",
			group, err,
		)
	}

	return GenerateKeypairResult{
		PublicKeyLocation:  s.Location(),
		PrivateKeyLocation: s.KeysLocation(),
		Keypair: KeypairMetadata{
			Group:            group,
			PublicKey:        pubKey,
			PrivateKeyStatus: PrivateKeyValid,
		},
	}, nil
}

// InspectKeypair reports the safe status of a group's public and private keys
// without writing, prompting, or returning private-key material.
func (s *Service) InspectKeypair(group string) (KeypairMetadata, error) {
	// Normalize the group before looking it up.
	var err error
	group, err = NormalizeGroupName(group)
	if err != nil {
		return KeypairMetadata{}, err
	}

	// Retrieve the group's public key from the repository.
	publicKey, exists, err := s.params.Repository.GetPublicKey(group)
	if err != nil {
		return KeypairMetadata{}, err
	}
	if !exists {
		return KeypairMetadata{}, fmt.Errorf(
			"group %q has no public key: %w", group, ErrGroupNotFound,
		)
	}

	// Start with the safest status until usable private-key material is found.
	metadata := KeypairMetadata{
		Group:            group,
		PublicKey:        publicKey,
		PrivateKeyStatus: PrivateKeyNotAvailable,
	}
	if s.params.PrivateKeyService == nil {
		return metadata, nil
	}

	// Resolve and validate the private key without exposing its contents.
	privateKey, err := s.params.PrivateKeyService.Resolve(group)
	if err != nil {
		if errors.Is(err, privatekey.ErrNotAvailable) {
			return metadata, nil
		}
		return KeypairMetadata{}, fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}

	if privateKey.Value == "" ||
		s.params.Cipher.ValidateKeypair(publicKey, privateKey.Value) != nil {
		metadata.PrivateKeyStatus = PrivateKeyInvalid
		return metadata, nil
	}

	metadata.PrivateKeyStatus = PrivateKeyValid
	return metadata, nil
}

// ListKeypairs returns metadata for every group that declares a public key,
// validating each group's available private key without exposing key material.
// A missing store yields an empty slice. It is the store-level counterpart to
// InspectKeypair a workspace-wide validator uses to surface a broken or
// unavailable keypair no single reference reveals.
func (s *Service) ListKeypairs() ([]KeypairMetadata, error) {
	records, err := s.params.Repository.ListKeypairs()
	if err != nil {
		return nil, err
	}

	out := make([]KeypairMetadata, 0, len(records))
	for _, record := range records {
		metadata, err := s.InspectKeypair(record.Group)
		if err != nil {
			return nil, err
		}
		out = append(out, metadata)
	}
	return out, nil
}

// Keypairs is an alias for ListKeypairs for backward compatibility.
func (s *Service) Keypairs() ([]KeypairMetadata, error) {
	return s.ListKeypairs()
}

// RotateKeypair replaces a group's identity and re-encrypts its complete set of
// values under the new public key. The current private key must be available so
// every stored value can be decrypted and re-encrypted, so rotation fails closed
// when it is missing. The new private key is delivered before the new public
// state is committed, mirroring generation's safe write order.
func (s *Service) RotateKeypair(group string) (RotateKeypairResult, error) {
	// Normalize the group before using it in storage or key paths.
	var err error
	group, err = NormalizeGroupName(group)
	if err != nil {
		return RotateKeypairResult{}, err
	}

	// Require the group to already have an identity.
	_, exists, err := s.params.Repository.GetPublicKey(group)
	if err != nil {
		return RotateKeypairResult{}, err
	}
	if !exists {
		return RotateKeypairResult{}, fmt.Errorf(
			"group %q has no public key; run 'envx keypair generate %s' first: %w",
			group, group, ErrGroupNotFound,
		)
	}

	// Resolve the current private key; rotation must decrypt the whole group.
	oldKey, err := s.resolveRotationKey(group)
	if err != nil {
		return RotateKeypairResult{}, err
	}

	// Enforce the destination provenance rule before generating new material.
	if err := s.checkRotationDestination(group, oldKey); err != nil {
		return RotateKeypairResult{}, err
	}

	// Decrypt the complete group in memory before generating the replacement.
	plaintextSecrets, err := s.decryptGroup(group, oldKey.Value)
	if err != nil {
		return RotateKeypairResult{}, err
	}

	// Generate and validate the replacement identity before touching the store.
	pair, err := s.params.Cipher.Keypair()
	if err != nil {
		return RotateKeypairResult{}, fmt.Errorf(
			"generating keypair for group %q: %w", group, err,
		)
	}
	newPub, newPriv := pair.PublicKey, pair.PrivateKey
	if newPub == "" || newPriv == "" {
		return RotateKeypairResult{}, errors.New("cipher generated an incomplete keypair")
	}
	if err := s.params.Cipher.ValidateKeypair(newPub, newPriv); err != nil {
		return RotateKeypairResult{}, fmt.Errorf("generated keypair is invalid: %w", err)
	}

	// Stage the re-encrypted values in memory.
	newRecords, references, err := s.reencryptGroup(group, newPub, plaintextSecrets)
	if err != nil {
		return RotateKeypairResult{}, err
	}

	// Deliver the new private key before committing the new public state.
	if err := s.params.PrivateKeyService.Set(group, newPriv); err != nil {
		return RotateKeypairResult{}, fmt.Errorf(
			"writing private key for group %q: %w", group, err,
		)
	}

	// Commit the new public key.
	if err := s.params.Repository.SetPublicKey(group, newPub); err != nil {
		return RotateKeypairResult{}, fmt.Errorf(
			"committing public key for group %q: %w", group, err,
		)
	}

	// Commit the re-encrypted values.
	if batchRepo, ok := s.params.Repository.(BatchRepository); ok {
		if err := batchRepo.SetSecrets(newRecords); err != nil {
			return RotateKeypairResult{}, fmt.Errorf(
				"committing secrets for group %q: %w", group, err,
			)
		}
	} else {
		for _, rec := range newRecords {
			if err := s.params.Repository.SetSecret(rec); err != nil {
				return RotateKeypairResult{}, fmt.Errorf(
					"committing secret %q in group %q: %w", rec.Key, rec.Group, err,
				)
			}
		}
	}

	return RotateKeypairResult{
		PublicKeyLocation:  s.Location(),
		PrivateKeyLocation: s.KeysLocation(),
		Keypair: KeypairMetadata{
			Group:            group,
			PublicKey:        newPub,
			PrivateKeyStatus: PrivateKeyValid,
		},
		Secrets: references,
	}, nil
}

// resolveRotationKey resolves the group's current private key, translating an
// unavailable key into a concise rotation failure.
func (s *Service) resolveRotationKey(group string) (privatekey.PrivateKey, error) {
	oldKey, err := s.params.PrivateKeyService.Resolve(group)
	if err != nil {
		if errors.Is(err, privatekey.ErrNotAvailable) {
			return privatekey.PrivateKey{}, fmt.Errorf(
				"cannot rotate group %q: its private key is not available", group,
			)
		}
		return privatekey.PrivateKey{}, fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}
	return oldKey, nil
}

// checkRotationDestination rejects rotation when the current private key came
// from a higher-priority source (e.g., environment variable), which would
// shadow any newly stored key.
func (s *Service) checkRotationDestination(
	group string, oldKey privatekey.PrivateKey,
) error {
	if oldKey.Origin == "store" || oldKey.Origin == "" {
		return nil
	}
	return fmt.Errorf(
		"cannot rotate group %q: its current private key "+
			"came from %s, which has higher lookup priority and would shadow the new "+
			"key; rotate with an explicit private-key destination instead",
		group, oldKey.Origin,
	)
}

// decryptGroup decrypts every stored value in one group with its current private
// key, returning the plaintext values in document order.
func (s *Service) decryptGroup(
	group, privateKey string,
) ([]SecretRecord, error) {
	allSecrets, err := s.params.Repository.ListSecrets()
	if err != nil {
		return nil, err
	}

	var plaintext []SecretRecord
	for _, secret := range allSecrets {
		if !strings.EqualFold(secret.Group, group) {
			continue
		}
		if !s.codec.isCiphertext(secret.Ciphertext) {
			return nil, fmt.Errorf(
				"secret %q in group %q is not encrypted; encrypt the group before rotating",
				secret.Key, secret.Group,
			)
		}
		alg, payload, err := s.codec.decode(secret.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf(
				"decoding secret %q in group %q: %w", secret.Key, secret.Group, err,
			)
		}
		if alg != s.params.Cipher.Algorithm() {
			return nil, fmt.Errorf(
				"secret %q in group %q uses algorithm %q, but the configured cipher is %q",
				secret.Key, secret.Group, alg, s.params.Cipher.Algorithm(),
			)
		}
		val, err := s.params.Cipher.Decrypt(payload, privateKey)
		if err != nil {
			return nil, fmt.Errorf(
				"decrypting secret %q in group %q: %w", secret.Key, secret.Group, err,
			)
		}
		plaintext = append(plaintext, SecretRecord{
			Group: group, Key: secret.Key, Ciphertext: val,
		})
	}
	return plaintext, nil
}

// reencryptGroup re-encrypts every plaintext value under the new public key,
// returning the new records and references.
func (s *Service) reencryptGroup(
	group, publicKey string,
	plaintextSecrets []SecretRecord,
) ([]SecretRecord, []SecretReference, error) {
	records := make([]SecretRecord, 0, len(plaintextSecrets))
	references := make([]SecretReference, 0, len(plaintextSecrets))
	for _, secret := range plaintextSecrets {
		native, err := s.params.Cipher.Encrypt(secret.Ciphertext, publicKey)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"encrypting secret %q in group %q: %w", secret.Key, group, err,
			)
		}
		ciphertext, err := s.codec.encode(s.params.Cipher.Algorithm(), native)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"encoding secret %q in group %q: %w", secret.Key, group, err,
			)
		}
		records = append(records, SecretRecord{
			Group: group,
			Key:   secret.Key,

			Ciphertext: ciphertext,
		})
		references = append(references, SecretReference{
			Group: group, Key: secret.Key,
		})
	}
	return records, references, nil
}
