package filestore

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/secrets"
)

func TestExporterWriteSecrets(t *testing.T) {
	t.Parallel()

	exporter, err := NewExporter(ExporterParams{})
	if err != nil {
		t.Fatalf("NewExporter(): %v", err)
	}
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	records := []secrets.SecretRecord{
		{Group: "production", Key: "token", Ciphertext: "encrypted-age:abc"},
		{Group: "shared", Key: "api_key", Ciphertext: "encrypted-age:def"},
	}

	if err := exporter.WriteSecrets(path, records); err != nil {
		t.Fatalf("WriteSecrets(): %v", err)
	}

	store, err := New(Params{Path: path})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	for _, want := range records {
		got, found, err := store.GetSecret(want.Group, want.Key)
		if err != nil || !found {
			t.Fatalf("GetSecret(%s/%s) = found %v, err %v", want.Group, want.Key, found, err)
		}
		if got.Ciphertext != want.Ciphertext {
			t.Errorf("Ciphertext = %q, want %q", got.Ciphertext, want.Ciphertext)
		}
	}
}

func TestExporterRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	exporter, err := NewExporter(ExporterParams{})
	if err != nil {
		t.Fatalf("NewExporter(): %v", err)
	}
	if err := exporter.WriteSecrets("", nil); err == nil {
		t.Error("WriteSecrets() accepted an empty path")
	}
}
