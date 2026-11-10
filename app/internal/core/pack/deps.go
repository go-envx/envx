package pack

import (
	"io"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// SecretStore reads raw stored secret groups and serializes a selection of
// them, never decrypting values.
type SecretStore interface {
	// Read returns one group; a missing group yields Exists false.
	Read(group string) (secrets.Snapshot, error)
	// Export serializes groups as a run-only store without public keys.
	Export(out io.Writer, groups []secrets.Group) error
}

// Dependencies are the adapters a Service uses; Secrets is required.
type Dependencies struct {
	// Secrets reads the workspace store and exports the bundle store.
	Secrets SecretStore
}
