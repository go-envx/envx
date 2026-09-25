package filestore

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/utils/filex"
	"github.com/go-envx/envx/app/internal/utils/git"
)

var _ privatekey.Repository = (*Repository)(nil)

const (
	// defaultFilename is the default private-key file name.
	defaultFilename = "envx.keys"
	// defaultOrigin is the provenance identifier reported by Repository.
	defaultOrigin = "store"
)

// Params configures the local filesystem key store.
type Params struct {
	// Path is the absolute or relative filesystem path of the private-key file.
	Path string
}

// Repository implements privatekey.Repository for a local file.
type Repository struct {
	path string
}

// New constructs a file-backed private key repository.
func New(params Params) (*Repository, error) {
	path := strings.TrimSpace(params.Path)
	if path == "" {
		path = defaultFilename
	}
	return &Repository{path: path}, nil
}

// Origin returns the provenance identifier for the repository.
func (r *Repository) Origin() string {
	return defaultOrigin
}

// GetPrivateKey retrieves a private key from the local key file.
func (r *Repository) GetPrivateKey(group string) (key string, found bool, err error) {
	if err := privatekey.ValidateGroup(group); err != nil {
		return "", false, err
	}

	data, err := filex.Read(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading private keys %s: %w", r.path, err)
	}

	parsed, err := parseDocument(string(data), r.path)
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", privatekey.ErrInvalidKey, err)
	}
	key, found = parsed.lookup(group)
	if !found {
		return "", false, nil
	}
	return key, true, nil
}

// SetPrivateKey adds or updates a group's private key in the local file.
func (r *Repository) SetPrivateKey(group, privateKey string) error {
	if err := privatekey.ValidateEntry(group, privateKey); err != nil {
		return err
	}

	if err := git.EnsureIgnored(r.path); err != nil {
		return err
	}

	data, err := filex.Read(r.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading private keys %s: %w", r.path, err)
	}
	content := ""
	if err == nil {
		content = string(data)
	}
	parsed, err := parseDocument(content, r.path)
	if err != nil {
		return fmt.Errorf("%w: %w", privatekey.ErrInvalidKey, err)
	}
	updated := parsed.upsert(group, privateKey)

	if err := filex.WriteAtomicPrivate(r.path, []byte(updated)); err != nil {
		return err
	}
	return nil
}
