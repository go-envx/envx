package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
	"github.com/go-envx/envx/app/pkg/file"
)

// GenerateKeypair creates a missing group identity and commits its public key
// only after the private-key destination has accepted the new private key.
func (m *Manager) GenerateKeypair(group string) (KeypairMetadata, error) {
	// Normalize the group before using it in storage or key paths.
	var err error
	group, err = normalizeGroupName(group)
	if err != nil {
		return KeypairMetadata{}, err
	}

	// Load the store and ensure this group does not already exist.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return KeypairMetadata{}, err
	}
	if _, exists := document.PublicKey(group); exists {
		return KeypairMetadata{}, fmt.Errorf(
			"group %q already has a public key", group,
		)
	}

	// Generate and validate both key halves before changing the document.
	pair, err := m.params.Cipher.Keypair()
	if err != nil {
		return KeypairMetadata{}, fmt.Errorf(
			"generating keypair for group %q: %w", group, err,
		)
	}
	if pair.PublicKey == "" || pair.PrivateKey == "" {
		return KeypairMetadata{}, errors.New("cipher generated an incomplete keypair")
	}
	if err := m.params.Cipher.ValidateKeypair(
		pair.PublicKey, pair.PrivateKey,
	); err != nil {
		return KeypairMetadata{}, fmt.Errorf("generated keypair is invalid: %w", err)
	}
	// Stage the public key until private-key delivery succeeds.
	if err := document.SetPublicKey(group, pair.PublicKey); err != nil {
		return KeypairMetadata{}, err
	}

	// Protect file-based destinations before writing private-key material.
	if m.params.PrivateKeyDestination == nil {
		return KeypairMetadata{}, errors.New("private-key destination is nil")
	}
	if keysPath, isFile := privatekey.FilePath(
		m.params.PrivateKeyDestination,
	); isFile {
		if err := ensureGitIgnored(keysPath); err != nil {
			return KeypairMetadata{}, err
		}
	}
	// Deliver the private key before committing its matching public key.
	if err := m.params.PrivateKeyDestination.Write(group, pair.PrivateKey); err != nil {
		return KeypairMetadata{}, fmt.Errorf(
			"writing private key for group %q: %w", group, err,
		)
	}
	// Commit the staged public key after the private key was accepted.
	if err := document.Save(m.params.DefaultIndent); err != nil {
		return KeypairMetadata{}, fmt.Errorf(
			"private key for group %q was written, but secrets store %s was not "+
				"committed: %w; keep the private key and retry the operation",
			group, m.params.SecretsPath, err,
		)
	}

	return KeypairMetadata{
		Group:            group,
		PublicKey:        pair.PublicKey,
		PrivateKeyStatus: PrivateKeyValid,
	}, nil
}

// InspectKeypair reports the safe status of a group's public and private keys
// without writing, prompting, or returning private-key material.
func (m *Manager) InspectKeypair(group string) (KeypairMetadata, error) {
	// Normalize the group before looking it up.
	var err error
	group, err = normalizeGroupName(group)
	if err != nil {
		return KeypairMetadata{}, err
	}

	// Load the store and retrieve the group's public key.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return KeypairMetadata{}, err
	}
	publicKey, exists := document.PublicKey(group)
	if !exists {
		return KeypairMetadata{}, fmt.Errorf("group %q has no public key", group)
	}

	// Start with the safest status until usable private-key material is found.
	metadata := KeypairMetadata{
		Group:            group,
		PublicKey:        publicKey,
		PrivateKeyStatus: PrivateKeyNotAvailable,
	}
	if m.params.PrivateKeyResolver == nil {
		return metadata, nil
	}

	// Resolve and validate the private key without exposing its contents.
	privateKey, err := m.params.PrivateKeyResolver.Resolve(group)
	if err != nil {
		if errors.Is(err, privatekey.ErrNotAvailable) {
			return metadata, nil
		}
		return KeypairMetadata{}, fmt.Errorf(
			"resolving private key for group %q: %w", group, err,
		)
	}
	if privateKey.Value == "" ||
		m.params.Cipher.ValidateKeypair(publicKey, privateKey.Value) != nil {
		metadata.PrivateKeyStatus = PrivateKeyInvalid
		return metadata, nil
	}

	metadata.PrivateKeyStatus = PrivateKeyValid
	return metadata, nil
}

// RotateKeypair replaces a group's identity and re-encrypts its complete set of
// values under the new public key. The current private key must be available so
// every stored value can be decrypted and re-encrypted, so rotation fails closed
// when it is missing. The new private key is delivered before the new public
// state is committed, mirroring generation's safe write order. When the private
// key is file-backed, the previous key file is preserved as rollback material
// until the store commit succeeds; a failed commit returns an actionable recovery
// error that points at the preserved key without exposing any private material.
func (m *Manager) RotateKeypair(group string) (UpdateResult, error) {
	// Normalize the group before using it in storage or key paths.
	group, err := normalizeGroupName(group)
	if err != nil {
		return UpdateResult{}, err
	}

	// Load the store and require the group to already have an identity.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return UpdateResult{}, err
	}
	if _, exists := document.PublicKey(group); !exists {
		return UpdateResult{}, fmt.Errorf(
			"group %q has no public key; run 'envx keypair generate %s' first",
			group, group,
		)
	}

	// Resolve the current private key; rotation must decrypt the whole group.
	oldKey, err := m.resolveRotationKey(group)
	if err != nil {
		return UpdateResult{}, err
	}

	// Enforce the destination provenance rule before generating new material.
	if err := m.checkRotationDestination(group, oldKey); err != nil {
		return UpdateResult{}, err
	}

	// Decrypt the complete group in memory before generating the replacement.
	plaintextSecrets, err := m.decryptGroup(document, group, oldKey.Value)
	if err != nil {
		return UpdateResult{}, err
	}

	// Generate and validate the replacement identity before touching the store.
	pair, err := m.params.Cipher.Keypair()
	if err != nil {
		return UpdateResult{}, fmt.Errorf(
			"generating keypair for group %q: %w", group, err,
		)
	}
	if pair.PublicKey == "" || pair.PrivateKey == "" {
		return UpdateResult{}, errors.New("cipher generated an incomplete keypair")
	}
	if err := m.params.Cipher.ValidateKeypair(
		pair.PublicKey, pair.PrivateKey,
	); err != nil {
		return UpdateResult{}, fmt.Errorf("generated keypair is invalid: %w", err)
	}

	// Stage the new public key and re-encrypted values in memory.
	references, err := m.reencryptGroup(document, group, pair.PublicKey, plaintextSecrets)
	if err != nil {
		return UpdateResult{}, err
	}

	// Preserve file-backed rollback material before overwriting the old key.
	backupPath, err := m.prepareRotationRollback()
	if err != nil {
		return UpdateResult{}, err
	}

	// Deliver the new private key before committing the new public state.
	if err := m.params.PrivateKeyDestination.Write(group, pair.PrivateKey); err != nil {
		return UpdateResult{}, fmt.Errorf(
			"writing private key for group %q: %w", group, err,
		)
	}

	// Commit the new public key and re-encrypted values after the private handoff.
	if err := document.Save(m.params.DefaultIndent); err != nil {
		return UpdateResult{}, m.rotationCommitError(group, backupPath, err)
	}

	// Remove rollback material once the rotation has fully committed.
	removeRotationRollback(backupPath)

	return UpdateResult{
		Keypairs: []KeypairMetadata{{
			Group:            group,
			PublicKey:        pair.PublicKey,
			PrivateKeyStatus: PrivateKeyValid,
		}},
		Secrets: references,
	}, nil
}

// resolveRotationKey resolves the group's current private key, translating an
// unavailable key into a concise rotation failure.
func (m *Manager) resolveRotationKey(group string) (privatekey.PrivateKey, error) {
	oldKey, err := m.params.PrivateKeyResolver.Resolve(group)
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

// checkRotationDestination rejects an implicit local key file when the current
// private key came from a higher-priority source. Writing the new key to the file
// would leave it shadowed by the environment variable or explicit source that
// supplied the old key, so the caller must select an explicit destination.
func (m *Manager) checkRotationDestination(
	group string, oldKey privatekey.PrivateKey,
) error {
	keysPath, isFile := privatekey.FilePath(m.params.PrivateKeyDestination)
	if !isFile || oldKey.Origin == keysPath {
		return nil
	}
	return fmt.Errorf(
		"cannot rotate group %q into the local key file %s: its current private key "+
			"came from %s, which has higher lookup priority and would shadow the new "+
			"key; rotate with an explicit private-key destination instead",
		group, keysPath, oldKey.Origin,
	)
}

// decryptGroup decrypts every stored value in one group with its current private
// key, returning the plaintext values in document order.
func (m *Manager) decryptGroup(
	document *store.Document, group, privateKey string,
) ([]store.Secret, error) {
	var plaintext []store.Secret
	for _, secret := range document.Secrets() {
		if !strings.EqualFold(secret.Group, group) {
			continue
		}
		if !envelope.IsCiphertext(secret.Value) {
			return nil, fmt.Errorf(
				"secret %q in group %q is not encrypted; encrypt the group before rotating",
				secret.Key, secret.Group,
			)
		}
		value, err := m.decryptStoredValue(secret, privateKey)
		if err != nil {
			return nil, err
		}
		plaintext = append(plaintext, store.Secret{
			Group: group, Key: secret.Key, Value: value,
		})
	}
	return plaintext, nil
}

// reencryptGroup stages the new public key and re-encrypts every plaintext value
// under it, returning the re-encrypted identities.
func (m *Manager) reencryptGroup(
	document *store.Document,
	group, publicKey string,
	plaintextSecrets []store.Secret,
) ([]SecretReference, error) {
	if err := document.SetPublicKey(group, publicKey); err != nil {
		return nil, err
	}
	references := make([]SecretReference, 0, len(plaintextSecrets))
	for _, secret := range plaintextSecrets {
		ciphertext, err := m.encryptStoredValue(document, secret)
		if err != nil {
			return nil, err
		}
		if err := document.SetSecret(group, secret.Key, ciphertext); err != nil {
			return nil, err
		}
		references = append(references, SecretReference{
			Group: group, Key: secret.Key,
		})
	}
	return references, nil
}

// prepareRotationRollback snapshots a file-backed private key so the previous key
// survives a failed store commit. It returns the backup path, or an empty path
// when the destination keeps no local file to preserve.
func (m *Manager) prepareRotationRollback() (string, error) {
	keysPath, isFile := privatekey.FilePath(m.params.PrivateKeyDestination)
	if !isFile {
		return "", nil
	}
	if err := ensureGitIgnored(keysPath); err != nil {
		return "", err
	}

	data, err := file.Read(keysPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading private keys %s: %w", keysPath, err)
	}

	backupPath := keysPath + backupSuffix
	if err := ensureGitIgnored(backupPath); err != nil {
		return "", err
	}
	if err := file.WriteAtomicPrivate(backupPath, data); err != nil {
		return "", fmt.Errorf(
			"preserving previous private key at %s: %w", backupPath, err,
		)
	}
	return backupPath, nil
}

// rotationCommitError describes a second-write failure without exposing private
// material, pointing the operator at any preserved rollback key.
func (m *Manager) rotationCommitError(group, backupPath string, cause error) error {
	if backupPath != "" {
		return fmt.Errorf(
			"the new private key for group %q was written, but the secrets store %s "+
				"was not committed: %w; the previous private key is preserved at %s - "+
				"restore it to keep decrypting the current store, then retry the rotation",
			group, m.params.SecretsPath, cause, backupPath,
		)
	}
	return fmt.Errorf(
		"the new private key for group %q was written, but the secrets store %s was "+
			"not committed: %w; retry the rotation with the new private key",
		group, m.params.SecretsPath, cause,
	)
}

// removeRotationRollback deletes preserved rollback material after a successful
// rotation. A removal failure is not fatal because the rotation already committed.
func removeRotationRollback(backupPath string) {
	if backupPath == "" {
		return
	}
	_ = os.Remove(backupPath)
}

// backupSuffix names the preserved previous private-key file during rotation.
const backupSuffix = ".rotated.bak"

// ensureGitIgnored verifies a file is ignored by Git or adds a local rule before
// any private-key bytes are written. It leaves ignore files untouched when Git
// is unavailable. A repository ancestor's rule is sufficient.
func ensureGitIgnored(keysPath string) error {
	// Validate the path and prepare its parent directory.
	if keysPath == "" {
		return errors.New("private-key file path is empty")
	}

	dir := filepath.Dir(keysPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating private-key directory %s: %w", dir, err)
	}

	// Respect existing Git ignore rules, including repository-level rules.
	ignored, available, err := gitIgnores(dir, filepath.Base(keysPath))
	if err != nil {
		return err
	}
	if !available {
		return nil
	}
	if ignored {
		return nil
	}

	// Read the local ignore file before adding a rule.
	ignorePath := filepath.Join(dir, ".gitignore")
	data, err := file.Read(ignorePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", ignorePath, err)
	}
	if hasIgnoreRule(string(data), filepath.Base(keysPath)) {
		return nil
	}

	// Append a local rule so future private-key writes remain protected.
	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += filepath.Base(keysPath) + "\n"
	if err := file.WriteAtomic(ignorePath, []byte(content)); err != nil {
		return fmt.Errorf("protecting private-key file with %s: %w", ignorePath, err)
	}
	return nil
}

// gitIgnores asks Git's effective matcher whether name is ignored from dir.
// The second result reports whether Git was available to answer the query.
func gitIgnores(dir, name string) (ignored, available bool, err error) {
	//nolint:gosec // Git is fixed; name is only a path basename, never a command.
	cmd := exec.CommandContext(
		context.Background(), "git", "check-ignore", "--no-index", "--quiet", "--", name,
	)
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		return true, true, nil
	} else {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) &&
			(exitError.ExitCode() == 1 || exitError.ExitCode() == 128) {
			return false, true, nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return false, false, nil
		}
		return false, true, fmt.Errorf(
			"checking Git ignore for %s: %w", filepath.Join(dir, name), err,
		)
	}
}

// hasIgnoreRule reports whether a local ignore file already contains an exact
// rule for the private-key basename.
func hasIgnoreRule(content, name string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == name || line == "/"+name {
			return true
		}
	}
	return false
}
