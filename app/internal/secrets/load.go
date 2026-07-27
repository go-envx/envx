package secrets

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// secretsFile is the on-disk YAML shape of the secrets file: values nested by
// key-group then key, mirroring the document. loadStore flattens it into the
// reference-keyed store. Unknown fields (such as the public_keys block a later
// phase adds) are ignored on load.
type secretsFile struct {
	// Secrets maps a key-group name to its key/value entries.
	Secrets map[string]map[string]string `yaml:"secrets"`
}

// -------------------------------------------------------------------------------------

// loadStore reads and parses the secrets file at path. A missing file yields an
// empty store rather than an error, since secrets are optional and a workspace
// without them must still resolve; a malformed file is an error.
func loadStore(path string) (*store, error) {
	data, err := file.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &store{}, nil
		}
		return nil, fmt.Errorf("reading secrets %s: %w", path, err)
	}

	var sf secretsFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing secrets %s: %w", path, err)
	}
	return &store{secrets: sf.flatten()}, nil
}

// -------------------------------------------------------------------------------------

// flatten converts the nested on-disk shape into the reference-keyed map the
// store holds, so lookups index by a single reference rather than two steps.
func (sf secretsFile) flatten() map[reference]string {
	out := make(map[reference]string)
	for group, entries := range sf.Secrets {
		for key, value := range entries {
			out[reference{group: group, key: key}] = value
		}
	}
	return out
}
