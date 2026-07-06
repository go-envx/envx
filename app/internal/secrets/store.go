package secrets

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// Store is the parsed secrets file: secret values grouped by key-group. In this
// phase values are stored verbatim (plaintext); a later phase will store
// ciphertext and decrypt on lookup, without changing this type's shape.
type Store struct {
	// secrets maps a key-group name to its key/value entries.
	secrets map[string]map[string]string
}

// -------------------------------------------------------------------------------------

// storeFile is the on-disk YAML shape of the secrets file. Unknown fields (such
// as the public_keys block a later phase adds) are ignored on load.
type storeFile struct {
	// Secrets maps a key-group name to its key/value entries.
	Secrets map[string]map[string]string `yaml:"secrets"`
}

// -------------------------------------------------------------------------------------

// Load reads and parses the secrets file at path. A missing file yields an empty
// store rather than an error, since secrets are optional and a workspace without
// them must still resolve; a malformed file is an error.
func Load(path string) (*Store, error) {
	data, err := file.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Store{}, nil
		}
		return nil, fmt.Errorf("reading secrets %s: %w", path, err)
	}

	var sf storeFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing secrets %s: %w", path, err)
	}
	return &Store{secrets: sf.Secrets}, nil
}

// -------------------------------------------------------------------------------------

// Lookup returns the value of key within group and whether it was present.
func (s *Store) Lookup(group, key string) (string, bool) {
	entries, ok := s.secrets[group]
	if !ok {
		return "", false
	}
	v, ok := entries[key]
	return v, ok
}
