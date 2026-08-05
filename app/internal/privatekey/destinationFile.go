package privatekey

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// fileDestination stores private keys in a local NAME=value file.
type fileDestination struct {
	// path is the private key file to update.
	path string
}

// -------------------------------------------------------------------------------------

// NewFileDestination creates a private local-file destination.
func NewFileDestination(path string) Destination {
	return fileDestination{path: path}
}

// -------------------------------------------------------------------------------------

// Path reports the private key file this destination updates.
func (d fileDestination) Path() string {
	return d.path
}

// -------------------------------------------------------------------------------------

// Write adds or updates group in the private key file and keeps existing entries.
func (d fileDestination) Write(group, privateKey string) error {
	if err := validateEntry(group, privateKey); err != nil {
		return err
	}
	if d.path == "" {
		return errors.New("private-key file path is empty")
	}

	data, err := file.Read(d.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading private keys %s: %w", d.path, err)
	}
	content := ""
	if err == nil {
		content = string(data)
	}
	parsed, err := parseKeyFile(content, d.path)
	if err != nil {
		return err
	}
	updated := parsed.upsert(group, privateKey)

	if dir := filepath.Dir(d.path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("creating private-key directory %s: %w", dir, err)
		}
	}
	if err := file.WriteAtomicPrivate(d.path, []byte(updated)); err != nil {
		return err
	}
	return nil
}
