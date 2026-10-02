package filestore

import "github.com/go-envx/envx/app/internal/features/secrets"

// ExporterParams configures the secrets store exporter.
type ExporterParams struct {
	DefaultIndent int
}

// Exporter writes secret records into standalone YAML secrets stores.
type Exporter struct {
	defaultIndent int
}

// NewExporter constructs a secrets store exporter.
func NewExporter(params ExporterParams) (*Exporter, error) {
	return &Exporter{defaultIndent: params.DefaultIndent}, nil
}

// WriteSecrets writes records into the secrets store at path in one atomic write.
func (e *Exporter) WriteSecrets(path string, records []secrets.SecretRecord) error {
	store, err := New(Params{Path: path, DefaultIndent: e.defaultIndent})
	if err != nil {
		return err
	}
	return store.SetSecrets(records)
}
