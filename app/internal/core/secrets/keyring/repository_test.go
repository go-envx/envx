package keyring_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/secrets/keyring"
	"github.com/go-envx/envx/app/internal/lib/crypt"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/testutil"
)

// hostEnv returns a host lookup over vars.
func hostEnv(vars map[string]string) *hostenv.Lookup {
	environ := make([]string, 0, len(vars))
	for name, value := range vars {
		environ = append(environ, name+"="+value)
	}
	return hostenv.From(environ)
}

// newRepository returns a keyring repository over a temp key file seeded with
// content.
func newRepository(
	t *testing.T, content string, vars map[string]string,
) (repo *keyring.Repository, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "envx.local.keys")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := keyring.New(path, hostEnv(vars))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return repo, path
}

// TestResolvePrecedence verifies the group variable beats the bundle, which
// beats the key file.
func TestResolvePrecedence(t *testing.T) {
	t.Parallel()
	const file = "# local keys\nAPI=file-key\n"

	tests := []struct {
		name       string
		vars       map[string]string
		wantValue  string
		wantOrigin string
	}{
		{"file", nil, "file-key", secrets.OriginStore},
		{
			"bundle",
			map[string]string{"ENVX_PRIVATE_KEY": "web=other\napi=bundle-key"},
			"bundle-key", "ENVX_PRIVATE_KEY",
		},
		{
			"bundle without group falls through",
			map[string]string{"ENVX_PRIVATE_KEY": "WEB=other"},
			"file-key", secrets.OriginStore,
		},
		{
			"specific",
			map[string]string{
				"ENVX_PRIVATE_KEY":     "API=bundle-key",
				"ENVX_PRIVATE_KEY_API": "specific-key",
			},
			"specific-key", "ENVX_PRIVATE_KEY_API",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newRepository(t, file, tt.vars)
			key, err := repo.Resolve("Api", "")
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if key.Value != tt.wantValue || key.Origin != tt.wantOrigin {
				t.Errorf("Resolve = %+v, want %s from %s", key, tt.wantValue, tt.wantOrigin)
			}
		})
	}
}

// TestResolveOriginNamesLocalEnvFile verifies a file override names the file
// and a process override stays the bare variable name.
func TestResolveOriginNamesLocalEnvFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "envx.local.env")
	content := []byte("ENVX_PRIVATE_KEY_API=file-key\n")
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		environ []string
		want    string
	}{
		{"file", nil, "ENVX_PRIVATE_KEY_API in " + file},
		{"process", []string{"ENVX_PRIVATE_KEY_API=proc-key"}, "ENVX_PRIVATE_KEY_API"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lookup := hostenv.NewLookup(hostenv.Options{
				File:    file,
				Environ: func() []string { return tt.environ },
			})
			repo, err := keyring.New(filepath.Join(t.TempDir(), "envx.local.keys"), lookup)
			if err != nil {
				t.Fatal(err)
			}
			key, err := repo.Resolve("api", "")
			if err != nil || key.Origin != tt.want {
				t.Errorf("Resolve = %+v, %v; want origin %q", key, err, tt.want)
			}
		})
	}
}

// TestResolveFailures verifies missing keys are unavailable and malformed
// sources are invalid.
func TestResolveFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		vars    map[string]string
		target  error
	}{
		{"missing file", "", nil, secrets.ErrKeyNotAvailable},
		{"missing group", "WEB=key\n", nil, secrets.ErrKeyNotAvailable},
		{"empty file", "\n", nil, secrets.ErrKeyNotAvailable},
		{"malformed file", "API\n", nil, secrets.ErrInvalidKey},
		{"duplicate entry", "API=a\napi=b\n", nil, secrets.ErrInvalidKey},
		{
			"empty specific", "API=a\n",
			map[string]string{"ENVX_PRIVATE_KEY_API": ""},
			secrets.ErrInvalidKey,
		},
		{
			"empty bundle", "API=a\n",
			map[string]string{"ENVX_PRIVATE_KEY": ""},
			secrets.ErrInvalidKey,
		},
		{
			"malformed bundle", "API=a\n",
			map[string]string{"ENVX_PRIVATE_KEY": "API="},
			secrets.ErrInvalidKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newRepository(t, tt.content, tt.vars)
			_, err := repo.Resolve("api", "pub")
			if !errors.Is(err, tt.target) {
				t.Errorf("Resolve error = %v, want %v", err, tt.target)
			}
		})
	}
}

// TestRetainPreservesExistingKeys verifies retaining a new key keeps every
// existing entry resolvable and writes an owner-only file.
func TestRetainPreservesExistingKeys(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "# keep me\nAPI=old-key\nWEB=web-key\n", nil)

	if err := repo.Retain("api", crypt.Keypair{
		PublicKey: "pub-new", PrivateKey: "new-key",
	}); err != nil {
		t.Fatalf("Retain: %v", err)
	}

	data, err := filex.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, line := range []string{"# keep me", "API=old-key", "WEB=web-key"} {
		if !strings.Contains(content, line+"\n") {
			t.Errorf("key file lost %q:\n%s", line, content)
		}
	}
	testutil.AssertPerm(t, path, 0o600)

	tests := []struct{ publicKey, want string }{
		{"pub-new", "new-key"},
		{"pub-old", "old-key"},
	}
	for _, tt := range tests {
		key, err := repo.Resolve("api", tt.publicKey)
		if err != nil || key.Value != tt.want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", tt.publicKey, key.Value, err, tt.want)
		}
	}

	_, err = repo.Resolve("api", "")
	if !errors.Is(err, secrets.ErrInvalidKey) {
		t.Errorf("ambiguous Resolve error = %v, want ErrInvalidKey", err)
	}
}

// TestRetainIgnoresKeysFile verifies the directory holds only the key file and
// a .gitignore naming it.
func TestRetainIgnoresKeysFile(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "", nil)

	if err := repo.Retain("api", crypt.Keypair{
		PublicKey: "pub", PrivateKey: "key",
	}); err != nil {
		t.Fatalf("Retain: %v", err)
	}
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{".gitignore", filepath.Base(path)}; !slices.Equal(names, want) {
		t.Errorf("directory holds %v, want %v", names, want)
	}
	data, err := filex.Read(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "envx.local.*\n"; string(data) != want {
		t.Errorf(".gitignore = %q, want %q", data, want)
	}
}

// TestRetainIgnoresCustomKeysFileByName verifies a key file outside the
// envx.local.* pattern is ignored by its exact name.
func TestRetainIgnoresCustomKeysFileByName(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "custom.keys")
	repo, err := keyring.New(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Retain("api", crypt.Keypair{
		PublicKey: "pub", PrivateKey: "key",
	}); err != nil {
		t.Fatalf("Retain: %v", err)
	}
	data, err := filex.Read(filepath.Join(filepath.Dir(path), ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "custom.keys\n"; string(data) != want {
		t.Errorf(".gitignore = %q, want %q", data, want)
	}
}

// TestRetainConcurrentKeepsEveryKey verifies parallel Retain calls drop no key.
func TestRetainConcurrentKeepsEveryKey(t *testing.T) {
	t.Parallel()
	repo, _ := newRepository(t, "", nil)

	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Go(func() {
			errs <- repo.Retain("api", crypt.Keypair{
				PublicKey:  fmt.Sprintf("pub-%d", i),
				PrivateKey: fmt.Sprintf("key-%d", i),
			})
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Retain: %v", err)
		}
	}
	for i := range workers {
		key, err := repo.Resolve("api", fmt.Sprintf("pub-%d", i))
		want := fmt.Sprintf("key-%d", i)
		if err != nil || key.Value != want {
			t.Errorf("Resolve(pub-%d) = %q, %v; want %q", i, key.Value, err, want)
		}
	}
}

// TestRetainAppendsAfterMissingNewline verifies an unterminated last line is
// kept intact.
func TestRetainAppendsAfterMissingNewline(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "API=old", nil)

	if err := repo.Retain("api", crypt.Keypair{
		PublicKey: "pub", PrivateKey: "key",
	}); err != nil {
		t.Fatalf("Retain: %v", err)
	}
	data, err := filex.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "API=old\nAPI/") ||
		!strings.HasSuffix(string(data), "=key\n") {
		t.Errorf("key file = %q", data)
	}
}

// TestRetainExistingEntryIsNotRewritten verifies retaining a held key leaves
// the file byte-for-byte unchanged.
func TestRetainExistingEntryIsNotRewritten(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "", nil)
	pair := crypt.Keypair{PublicKey: "pub", PrivateKey: "key"}
	if err := repo.Retain("api", pair); err != nil {
		t.Fatal(err)
	}
	before, err := filex.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	pair.PrivateKey = "other"
	if err := repo.Retain("api", pair); err != nil {
		t.Fatal(err)
	}
	after, err := filex.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("key file changed:\n%q\n%q", before, after)
	}
}

// TestRetainTightensPermissions verifies an existing wider file ends up 0600.
func TestRetainTightensPermissions(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "API=old\n", nil)
	//nolint:gosec // G302: deliberately loosened to verify Retain tightens it
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.Retain("api", crypt.Keypair{
		PublicKey: "pub", PrivateKey: "key",
	}); err != nil {
		t.Fatalf("Retain: %v", err)
	}
	testutil.AssertPerm(t, path, 0o600)
}

// TestRetainedKeysAreReadableFromBundle verifies a key file's contents work
// unchanged as an ENVX_PRIVATE_KEY bundle.
func TestRetainedKeysAreReadableFromBundle(t *testing.T) {
	t.Parallel()
	source, path := newRepository(t, "", nil)
	if err := source.Retain("api", crypt.Keypair{
		PublicKey: "pub", PrivateKey: "key",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := filex.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	repo, _ := newRepository(t, "", map[string]string{"ENVX_PRIVATE_KEY": string(data)})
	key, err := repo.Resolve("api", "pub")
	if err != nil || key.Value != "key" || key.Origin != "ENVX_PRIVATE_KEY" {
		t.Errorf("Resolve = %+v, %v", key, err)
	}
}

// TestRetainRejectsInvalidInput verifies invalid groups and keypairs are
// rejected before any file is written.
func TestRetainRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "", nil)

	tests := map[string]struct {
		group string
		pair  crypt.Keypair
	}{
		"invalid group":   {"a b", crypt.Keypair{PublicKey: "p", PrivateKey: "k"}},
		"missing public":  {"api", crypt.Keypair{PrivateKey: "k"}},
		"missing private": {"api", crypt.Keypair{PublicKey: "p"}},
		"line break":      {"api", crypt.Keypair{PublicKey: "p", PrivateKey: "a\nb"}},
	}
	for name, tt := range tests {
		if err := repo.Retain(tt.group, tt.pair); err == nil {
			t.Errorf("%s: Retain succeeded, want error", name)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("key file written after rejected input: %v", err)
	}
}
