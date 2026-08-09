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
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

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
	if err := document.Save(); err != nil {
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

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

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
