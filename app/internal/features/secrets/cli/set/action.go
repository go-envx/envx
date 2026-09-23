package set

import (
	"strings"

	"github.com/go-envx/envx/app/internal/config"
)

// actionParams identifies the one secret being written.
type actionParams struct {
	// Group identifies the case-insensitive key group.
	Group string
	// Key identifies the case-sensitive secret entry.
	Key string
	// Plaintext holds an explicitly supplied value, including an empty value.
	Plaintext *string
	// NoConfirm skips the interactive confirmation after hidden input.
	NoConfirm bool
}

// actionResult carries only safe mutation metadata to the renderer.
type actionResult struct {
	// Group is the normalized key group.
	Group string
	// Key is the stored secret entry name.
	Key string
	// StorePath is the workspace store that received the ciphertext.
	StorePath string
}

// execute reads one secret value and delegates encryption and storage to the
// root secrets manager.
func execute(
	p actionParams,
	in *config.Input,
	params readerParams,
) (actionResult, error) {
	// Resolve the workspace configuration and encryption settings.
	c, err := config.ResolveWorkspace(in)
	if err != nil {
		return actionResult{}, err
	}
	// Create the manager responsible for encrypted secret storage.
	manager, err := config.NewSecretsManager(c.Secrets, c.Cipher)
	if err != nil {
		return actionResult{}, err
	}
	// Function to read secret via terminal input.
	readSecret := func() (string, error) { return plaintextSource(p, params) }
	// Set the secret value via the secrets manager.
	if err := manager.Set(p.Group, p.Key, readSecret); err != nil {
		return actionResult{}, err
	}
	// Return secret metadata provided by secrets manager.
	return actionResult{
		Group:     strings.ToLower(p.Group),
		Key:       p.Key,
		StorePath: c.Secrets.SecretsPath,
	}, nil
}

// plaintextSource returns the explicit value or reads one value from the
// command's selected input stream.
func plaintextSource(p actionParams, params readerParams) (string, error) {
	if p.Plaintext != nil {
		return *p.Plaintext, nil
	}
	params.NoConfirm = p.NoConfirm
	return newReader(params).readSecret()
}
