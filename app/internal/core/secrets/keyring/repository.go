package keyring

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/lib/atomicfile"
	"github.com/go-envx/envx/app/internal/lib/crypt"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/git"
)

var _ secrets.Keys = (*Repository)(nil)

const (
	// EnvPrefix names the shared key bundle and prefixes group-specific keys.
	EnvPrefix = "ENVX_PRIVATE_KEY"

	defaultFilename = "envx.keys"
)

// Repository resolves private keys from the environment and a local key file,
// and retains new keys in that file.
type Repository struct {
	// path is the key file path.
	path string
	// lookupEnv reads environment overrides from the operation's snapshot.
	lookupEnv func(string) (string, bool)
}

// New constructs a keyring repository. An empty path uses "envx.keys"; a nil
// lookupEnv means no environment overrides.
func New(path string, lookupEnv func(string) (string, bool)) (*Repository, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultFilename
	}
	if lookupEnv == nil {
		lookupEnv = func(string) (string, bool) { return "", false }
	}
	return &Repository{path: path, lookupEnv: lookupEnv}, nil
}

// Location returns the key file path.
func (r *Repository) Location() string {
	return r.path
}

// Resolve returns a group's private key from ENVX_PRIVATE_KEY_<GROUP>, then the
// ENVX_PRIVATE_KEY bundle, then the key file. Stored keys are selected by
// publicKey when versions are retained.
func (r *Repository) Resolve(group, publicKey string) (secrets.PrivateKey, error) {
	name, err := secrets.NormalizeGroup(group)
	if err != nil {
		return secrets.PrivateKey{}, err
	}
	name = strings.ToUpper(name)

	specific := EnvPrefix + "_" + name
	if value, ok := r.lookupEnv(specific); ok {
		if value == "" {
			return secrets.PrivateKey{}, fmt.Errorf(
				"%w: environment variable %s is empty", secrets.ErrInvalidKey, specific,
			)
		}
		return secrets.PrivateKey{Value: value, Origin: specific}, nil
	}

	if bundle, ok := r.lookupEnv(EnvPrefix); ok {
		if bundle == "" {
			return secrets.PrivateKey{}, fmt.Errorf(
				"%w: environment variable %s is empty", secrets.ErrInvalidKey, EnvPrefix,
			)
		}
		key, found, err := selectKey(bundle, EnvPrefix, name, publicKey)
		if err != nil {
			return secrets.PrivateKey{}, err
		}
		if found {
			return secrets.PrivateKey{Value: key, Origin: EnvPrefix}, nil
		}
	}

	data, err := filex.Read(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return secrets.PrivateKey{}, unavailable(group)
	}
	if err != nil {
		return secrets.PrivateKey{}, fmt.Errorf(
			"reading private keys %s: %w", r.path, err,
		)
	}
	key, found, err := selectKey(string(data), r.path, name, publicKey)
	if err != nil {
		return secrets.PrivateKey{}, err
	}
	if !found {
		return secrets.PrivateKey{}, unavailable(group)
	}
	return secrets.PrivateKey{Value: key, Origin: secrets.OriginStore}, nil
}

// Retain durably stores a private key indexed by its public key without
// replacing any other stored key. It appends one line, so concurrent calls never
// drop each other's keys. The key file is Git-ignored before any key material
// is written, and the key file is owner-only.
func (r *Repository) Retain(group string, pair crypt.Keypair) error {
	name, err := secrets.NormalizeGroup(group)
	if err != nil {
		return err
	}
	if pair.PublicKey == "" || pair.PrivateKey == "" {
		return errors.New("keypair is incomplete")
	}
	if strings.ContainsAny(pair.PrivateKey, "\r\n") {
		return errors.New("private key contains a line break")
	}
	if err := git.EnsureIgnored(r.path); err != nil {
		return err
	}

	data, err := filex.Read(r.path)
	created := errors.Is(err, os.ErrNotExist)
	if err != nil && !created {
		return fmt.Errorf("reading private keys %s: %w", r.path, err)
	}
	doc, err := parseDocument(string(data), r.path)
	if err != nil {
		return fmt.Errorf("%w: %w", secrets.ErrInvalidKey, err)
	}
	name = strings.ToUpper(name)
	if doc.has(name, pair.PublicKey) {
		return nil
	}

	line := name + versionSeparator + fingerprint(pair.PublicKey) + "=" +
		pair.PrivateKey + "\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		line = "\n" + line
	}
	if err := appendPrivate(r.path, line, created); err != nil {
		return fmt.Errorf("writing private keys %s: %w", r.path, err)
	}
	return nil
}

// appendPrivate appends text to path in a single write, leaving the file
// owner-only and flushed to disk.
func appendPrivate(path, text string, created bool) (err error) {
	//nolint:gosec // G304: path is the configured key file
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := file.WriteString(text); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if created {
		return atomicfile.SyncDir(filepath.Dir(path))
	}
	return nil
}

// selectKey parses key content and selects a group's key, reporting malformed
// content as an invalid key.
func selectKey(
	content, source, group, publicKey string,
) (key string, found bool, err error) {
	doc, err := parseDocument(content, source)
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", secrets.ErrInvalidKey, err)
	}
	key, found, err = doc.lookup(group, publicKey)
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", secrets.ErrInvalidKey, err)
	}
	return key, found, nil
}

// unavailable reports a group with no private key in any source.
func unavailable(group string) error {
	return fmt.Errorf("%w for group %q", secrets.ErrKeyNotAvailable, group)
}
