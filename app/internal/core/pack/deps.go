package pack

import (
	"io"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// SecretReader reads raw stored secret groups without decrypting them.
type SecretReader interface {
	// Read returns one group; a missing group yields Exists false.
	Read(group string) (secrets.Snapshot, error)
}

// Dependencies are the adapters a Service uses; both are required.
type Dependencies struct {
	// Secrets reads the workspace store the bundle store is filtered from.
	Secrets SecretReader
	// ExportSecrets serializes the selected groups as a run-only store.
	ExportSecrets func(io.Writer, []secrets.Group) error
}
