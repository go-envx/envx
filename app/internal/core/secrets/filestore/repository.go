package filestore

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/atomicfile"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/git"
)

var _ secrets.Store = (*Repository)(nil)

// defaultIndent is the block indentation for documents with no indentation of
// their own.
const defaultIndent = 2

// Options configures serialization.
type Options struct {
	// Indent is the block indentation for new documents; values below 2 use 2.
	Indent int
}

// Repository persists secret groups in a YAML document. Construction performs
// no reads.
type Repository struct {
	// path is the secrets document path.
	path string
	// indent is the block indentation for documents without their own.
	indent int
}

// New constructs a repository for the document at path.
func New(path string, options Options) (*Repository, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("secrets repository path is empty")
	}
	return &Repository{path: path, indent: max(options.Indent, defaultIndent)}, nil
}

// Location returns the document path.
func (r *Repository) Location() string {
	return r.path
}

// Read returns one group, matched case-insensitively. A missing group or
// document yields Exists false.
func (r *Repository) Read(group string) (secrets.Snapshot, error) {
	if _, err := secrets.NormalizeGroup(group); err != nil {
		return secrets.Snapshot{}, err
	}
	doc, err := r.load()
	if err != nil {
		return secrets.Snapshot{}, err
	}
	return lookup(doc, group), nil
}

// List returns every group in first-seen document order.
func (r *Repository) List() ([]secrets.Snapshot, error) {
	doc, err := r.load()
	if err != nil {
		return nil, err
	}
	groups := doc.groups()
	out := make([]secrets.Snapshot, 0, len(groups))
	for _, group := range groups {
		out = append(out, snapshot(group))
	}
	return out, nil
}

// Commit locks the document, verifies that the group still matches expected,
// and atomically publishes the replacement's public key and values together.
func (r *Repository) Commit(
	expected secrets.Snapshot, replacement secrets.Group,
) error {
	if expected.Exists && !strings.EqualFold(expected.Group.Name, replacement.Name) {
		return fmt.Errorf(
			"replacement group %q does not match %q",
			replacement.Name, expected.Group.Name,
		)
	}
	lock, err := lockDir(r.path)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	doc, err := r.load()
	if err != nil {
		return err
	}
	current := lookup(doc, replacement.Name)
	if current.Exists != expected.Exists || current.Revision != expected.Revision {
		return fmt.Errorf("group %q: %w", replacement.Name, secrets.ErrConflict)
	}
	if err := doc.replace(replacement); err != nil {
		return err
	}
	data, err := doc.encode(r.indent)
	if err != nil {
		return err
	}
	if err := atomicfile.WritePrivate(r.path, data); err != nil {
		return fmt.Errorf("writing secrets %s: %w", r.path, err)
	}
	return nil
}

// Export writes the groups' stored values as a standalone secrets document
// without public keys, using the repository's indentation. It never decrypts
// values and does not touch the repository's document.
func (r *Repository) Export(out io.Writer, groups []secrets.Group) error {
	doc, err := parseDocument(nil)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if len(group.Values) == 0 {
			continue
		}
		if err := doc.replace(secrets.Group{
			Name: group.Name, Values: group.Values,
		}); err != nil {
			return err
		}
	}
	data, err := doc.encode(r.indent)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// lockDir takes the Git-ignored directory lock that guards path.
func lockDir(path string) (*atomicfile.FileLock, error) {
	lockPath := filepath.Join(filepath.Dir(path), workspace.LockFilename)
	if err := git.EnsureIgnored(lockPath); err != nil {
		return nil, err
	}
	return atomicfile.Lock(lockPath)
}

// load reads and parses the document; a missing file is an empty document.
func (r *Repository) load() (*document, error) {
	data, err := filex.Read(r.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading secrets %s: %w", r.path, err)
	}
	doc, err := parseDocument(data)
	if err != nil {
		return nil, fmt.Errorf("reading secrets %s: %w", r.path, err)
	}
	return doc, nil
}

// lookup returns a group's snapshot, matched case-insensitively.
func lookup(doc *document, name string) secrets.Snapshot {
	for _, group := range doc.groups() {
		if strings.EqualFold(group.Name, name) {
			return snapshot(group)
		}
	}
	return secrets.Snapshot{}
}

// snapshot wraps an existing group with its revision.
func snapshot(group secrets.Group) secrets.Snapshot {
	return secrets.Snapshot{Group: group, Revision: revision(group), Exists: true}
}

// revision digests a group's public key and values, independent of name case
// and document formatting. Each field is length-prefixed so field contents
// cannot be mistaken for boundaries.
func revision(group secrets.Group) string {
	hash := sha256.New()
	write := func(text string) {
		_ = binary.Write(hash, binary.BigEndian, uint64(len(text)))
		_, _ = io.WriteString(hash, text)
	}
	write(strings.ToLower(group.Name))
	write(group.PublicKey)
	for _, key := range sortedKeys(group.Values) {
		write(key)
		write(group.Values[key])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// sortedKeys returns a group's value keys in ascending order.
func sortedKeys(values map[string]string) []string {
	return slices.Sorted(maps.Keys(values))
}
