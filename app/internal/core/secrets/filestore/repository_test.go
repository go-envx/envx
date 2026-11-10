package filestore_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/secrets/filestore"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/str"
	"github.com/go-envx/envx/app/internal/testutil"
)

const (
	envelopeA = "encrypted-age:AAEC"
	envelopeB = "encrypted-age:AwQF"
)

var sampleDocument = str.Dedent(`
	# managed by envx
	public-keys:
	  API: pub-api

	secrets:
	  API:
	    # database token
	    TOKEN: encrypted-age:AAEC
	    OLD: legacy
	  web:
	    KEY: encrypted-age:AwQF
`) + "\n"

// newRepository returns a repository over a temp document seeded with content.
func newRepository(
	t *testing.T, content string,
) (repo *filestore.Repository, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "secrets.yaml")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := filestore.New(path, filestore.Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return repo, path
}

// read returns a file's contents.
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := filex.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestReadMissingDocument verifies a missing file reads as a missing group.
func TestReadMissingDocument(t *testing.T) {
	t.Parallel()
	repo, _ := newRepository(t, "")

	snap, err := repo.Read("api")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if snap.Exists || snap.Revision != "" {
		t.Errorf("snapshot = %+v, want missing", snap)
	}
}

// TestReadAndList verifies case-insensitive reads and first-seen group order.
func TestReadAndList(t *testing.T) {
	t.Parallel()
	repo, _ := newRepository(t, sampleDocument)

	snap, err := repo.Read("api")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !snap.Exists || snap.Group.Name != "API" || snap.Group.PublicKey != "pub-api" {
		t.Errorf("snapshot = %+v", snap)
	}
	if snap.Group.Values["TOKEN"] != envelopeA || snap.Group.Values["OLD"] != "legacy" {
		t.Errorf("values = %v", snap.Group.Values)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].Group.Name != "API" || list[1].Group.Name != "web" {
		t.Errorf("List = %+v", list)
	}
	if list[1].Group.PublicKey != "" || list[1].Group.Values["KEY"] != envelopeB {
		t.Errorf("web group = %+v", list[1].Group)
	}
}

// TestCommitTightensPermissions verifies commits leave the document owner-only.
func TestCommitTightensPermissions(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, sampleDocument)
	//nolint:gosec // G302: deliberately loosened to verify Commit tightens it
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Read("web")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(snap, snap.Group); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	testutil.AssertPerm(t, path, 0o600)
}

// TestCommitUsesIgnoredDirectoryLock verifies commits lock through a
// Git-ignored .envx.lock rather than a per-file lock.
func TestCommitUsesIgnoredDirectoryLock(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "")

	err := repo.Commit(secrets.Snapshot{}, secrets.Group{Name: "api", PublicKey: "pub"})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(filepath.Join(dir, ".envx.lock")); err != nil {
		t.Errorf("directory lock missing: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("per-file lock created: %v", err)
	}
	if ignore := read(t, filepath.Join(dir, ".gitignore")); !strings.Contains(
		ignore, ".envx.lock\n",
	) {
		t.Errorf(".gitignore = %q, want .envx.lock rule", ignore)
	}
}

// TestCommitRejectsStaleSnapshots verifies commits against an outdated or
// missing snapshot return ErrConflict.
func TestCommitRejectsStaleSnapshots(t *testing.T) {
	t.Parallel()
	repo, _ := newRepository(t, sampleDocument)
	stale, err := repo.Read("api")
	if err != nil {
		t.Fatal(err)
	}
	current := stale
	current.Group.Values = map[string]string{"TOKEN": envelopeB}
	if err := repo.Commit(stale, current.Group); err != nil {
		t.Fatalf("first Commit: %v", err)
	}

	err = repo.Commit(stale, secrets.Group{Name: "api", PublicKey: "x"})
	if !errors.Is(err, secrets.ErrConflict) {
		t.Errorf("stale Commit error = %v, want ErrConflict", err)
	}

	err = repo.Commit(secrets.Snapshot{}, secrets.Group{Name: "API", PublicKey: "x"})
	if !errors.Is(err, secrets.ErrConflict) {
		t.Errorf("create over existing group error = %v, want ErrConflict", err)
	}
}

// TestCommitRejectsStaleSnapshotWithEmbeddedNUL verifies values containing NUL
// cannot make a different group state hash to the stale revision.
func TestCommitRejectsStaleSnapshotWithEmbeddedNUL(t *testing.T) {
	t.Parallel()
	const joined = "secrets:\n  api:\n    A: \"x\\0B\\0y\"\n"
	const split = "secrets:\n  api:\n    A: x\n    B: y\n"

	joinedRepo, _ := newRepository(t, joined)
	splitRepo, _ := newRepository(t, split)
	joinedSnap, err := joinedRepo.Read("api")
	if err != nil {
		t.Fatal(err)
	}
	splitSnap, err := splitRepo.Read("api")
	if err != nil {
		t.Fatal(err)
	}
	if joinedSnap.Group.Values["A"] != "x\x00B\x00y" {
		t.Fatalf("fixture value = %q, want embedded NULs", joinedSnap.Group.Values["A"])
	}
	if joinedSnap.Revision == splitSnap.Revision {
		t.Fatal("distinct groups share a revision")
	}

	if err := joinedRepo.Commit(joinedSnap, splitSnap.Group); err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	err = joinedRepo.Commit(joinedSnap, secrets.Group{Name: "api", PublicKey: "x"})
	if !errors.Is(err, secrets.ErrConflict) {
		t.Errorf("stale Commit error = %v, want ErrConflict", err)
	}
}

// TestExportOmitsPublicKeys verifies exports contain only stored values, skip
// empty groups, and use the repository's indentation.
func TestExportOmitsPublicKeys(t *testing.T) {
	t.Parallel()
	groups := []secrets.Group{
		{Name: "api", PublicKey: "pub", Values: map[string]string{"TOKEN": envelopeA}},
		{Name: "empty", PublicKey: "pub"},
	}
	tests := []struct {
		// indent is the repository's configured indentation.
		indent int
		// want is the expected document.
		want string
	}{
		{indent: 0, want: "secrets:\n  api:\n    TOKEN: " + envelopeA + "\n"},
		{indent: 4, want: "secrets:\n    api:\n        TOKEN: " + envelopeA + "\n"},
	}
	for _, tt := range tests {
		repository, err := filestore.New(
			filepath.Join(t.TempDir(), "secrets.yaml"),
			filestore.Options{Indent: tt.indent},
		)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		var out bytes.Buffer
		if err := repository.Export(&out, groups); err != nil {
			t.Fatalf("Export: %v", err)
		}
		if out.String() != tt.want {
			t.Errorf("Export indent %d =\n%s\nwant\n%s", tt.indent, out.String(), tt.want)
		}
	}
}

// TestNewRejectsEmptyPath verifies a repository requires a document path.
func TestNewRejectsEmptyPath(t *testing.T) {
	t.Parallel()
	if _, err := filestore.New(" ", filestore.Options{}); err == nil {
		t.Error("New with empty path succeeded")
	}
}
