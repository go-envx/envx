package filestore

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/utils/filex"
)

var _ secrets.Repository = (*Repository)(nil)

// Params configures the YAML file store.
type Params struct {
	Path          string
	DefaultIndent int
}

// Repository implements secrets.Repository against a YAML document.
//
// Thread/Process Safety: Writes use atomic temporary file renaming via
// filex.WriteAtomic to prevent file corruption. In-process concurrent access
// and multi-process concurrent updates follow last-write-wins semantics.
// TODO: Implement advisory file locking (flock) to coordinate concurrent
// cross-process updates.
type Repository struct {
	path          string
	defaultIndent int
}

// New constructs a file-backed secrets repository.
func New(params Params) (*Repository, error) {
	path := strings.TrimSpace(params.Path)
	if path == "" {
		return nil, errors.New("secrets store path is empty")
	}
	indent := params.DefaultIndent
	if indent < 2 {
		indent = 2
	}
	return &Repository{
		path:          path,
		defaultIndent: indent,
	}, nil
}

// load reads the secrets document from disk, returning an empty document
// when the file does not exist.
func (r *Repository) load() (*document, error) {
	data, err := filex.Read(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return parseDocument(nil)
		}
		return nil, fmt.Errorf("reading secrets %s: %w", r.path, err)
	}
	doc, err := parseDocument(data)
	if err != nil {
		return nil, fmt.Errorf("reading secrets %s: %w", r.path, err)
	}
	return doc, nil
}

// save encodes and atomically writes the document to disk.
func (r *Repository) save(doc *document) error {
	data, err := doc.encode(r.defaultIndent)
	if err != nil {
		return err
	}
	if err := filex.WriteAtomic(r.path, data); err != nil {
		return fmt.Errorf("writing secrets %s: %w", r.path, err)
	}
	return nil
}

// Location satisfies secrets.Repository by returning the file store path.
func (r *Repository) Location() string {
	return r.path
}

// GetPublicKey satisfies secrets.Repository.
func (r *Repository) GetPublicKey(
	group string,
) (key string, found bool, err error) {
	doc, err := r.load()
	if err != nil {
		return "", false, err
	}
	key, found = doc.publicKey(group)
	return key, found, nil
}

// SetPublicKey satisfies secrets.Repository.
func (r *Repository) SetPublicKey(group, publicKey string) error {
	doc, err := r.load()
	if err != nil {
		return err
	}
	if err := doc.setPublicKey(group, publicKey); err != nil {
		return err
	}
	if err := r.save(doc); err != nil {
		return fmt.Errorf("saving public key for group %q: %w", group, err)
	}
	return nil
}

// ListKeypairs satisfies secrets.Repository.
func (r *Repository) ListKeypairs() ([]secrets.KeypairRecord, error) {
	doc, err := r.load()
	if err != nil {
		return nil, err
	}
	groups := doc.publicKeyGroups()
	records := make([]secrets.KeypairRecord, 0, len(groups))
	for _, group := range groups {
		key, ok := doc.publicKey(group)
		if ok {
			records = append(records, secrets.KeypairRecord{
				Group:     group,
				PublicKey: key,
			})
		}
	}
	return records, nil
}

// GetSecret satisfies secrets.Repository.
func (r *Repository) GetSecret(group, key string) (secrets.SecretRecord, bool, error) {
	doc, err := r.load()
	if err != nil {
		return secrets.SecretRecord{}, false, err
	}
	sec, found := doc.secret(group, key)
	if !found {
		return secrets.SecretRecord{}, false, nil
	}
	return secrets.SecretRecord{
		Group:      sec.Group,
		Key:        sec.Key,
		Ciphertext: sec.Value,
	}, true, nil
}

// SetSecret satisfies secrets.Repository.
func (r *Repository) SetSecret(record secrets.SecretRecord) error {
	doc, err := r.load()
	if err != nil {
		return err
	}
	if err := doc.setSecret(record.Group, record.Key, record.Ciphertext); err != nil {
		return err
	}
	if err := r.save(doc); err != nil {
		return fmt.Errorf("saving secret %q in group %q: %w", record.Key, record.Group, err)
	}
	return nil
}

// SetSecrets satisfies secrets.BatchRepository for atomic multi-entry persistence.
func (r *Repository) SetSecrets(records []secrets.SecretRecord) error {
	if len(records) == 0 {
		return nil
	}
	doc, err := r.load()
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := doc.setSecret(
			record.Group, record.Key, record.Ciphertext,
		); err != nil {
			return err
		}
	}
	if err := r.save(doc); err != nil {
		return fmt.Errorf("saving secrets store: %w", err)
	}
	return nil
}

// DeleteSecret satisfies secrets.Repository.
func (r *Repository) DeleteSecret(group, key string) error {
	doc, err := r.load()
	if err != nil {
		return err
	}
	deleted, err := doc.deleteSecret(group, key)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("%w: secret %q in group %q", secrets.ErrSecretNotFound, key, group)
	}
	if err := r.save(doc); err != nil {
		return fmt.Errorf("saving deletion of secret %q in group %q: %w", key, group, err)
	}
	return nil
}

// ListSecrets satisfies secrets.Repository.
func (r *Repository) ListSecrets() ([]secrets.SecretRecord, error) {
	doc, err := r.load()
	if err != nil {
		return nil, err
	}
	stored := doc.secrets()
	records := make([]secrets.SecretRecord, 0, len(stored))
	for _, sec := range stored {
		records = append(records, secrets.SecretRecord{
			Group:      sec.Group,
			Key:        sec.Key,
			Ciphertext: sec.Value,
		})
	}
	return records, nil
}
