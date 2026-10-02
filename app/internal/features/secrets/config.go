package secrets

import (
	"errors"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// Config is everything the workspace manifest configures for the secrets store.
type Config struct {
	// SecretsPath is the absolute path of the secrets store.
	SecretsPath string
	// KeysPath is the absolute path of the private-key file.
	KeysPath string
	// Cipher is the encryption algorithm name.
	Cipher string
	// DefaultIndent is the block indentation for rewritten store files.
	DefaultIndent int
}

// LoadConfig derives the secrets config from a loaded workspace.
func LoadConfig(ws *workspace.Workspace) (Config, error) {
	if ws == nil {
		return Config{}, errors.New("workspace is required")
	}
	return Config{
		SecretsPath:   ws.Secrets.SecretsPath,
		KeysPath:      ws.Secrets.KeysPath,
		Cipher:        ws.Secrets.Cipher,
		DefaultIndent: ws.Indent,
	}, nil
}
