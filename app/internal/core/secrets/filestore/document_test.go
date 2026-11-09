package filestore_test

import (
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/lib/str"
)

// TestCommitEditsInPlace verifies commits keep comments, blank lines, and key
// order while removing dropped keys and appending new ones.
func TestCommitEditsInPlace(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, sampleDocument)
	snap, err := repo.Read("api")
	if err != nil {
		t.Fatal(err)
	}

	err = repo.Commit(snap, secrets.Group{
		Name:      "api",
		PublicKey: "pub-new",
		Values:    map[string]string{"TOKEN": envelopeB, "NEW": "plain"},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	want := str.Dedent(`
		# managed by envx
		public-keys:
		  API: pub-new

		secrets:
		  API:
		    # database token
		    TOKEN: encrypted-age:AwQF
		    NEW: plain
		  web:
		    KEY: encrypted-age:AwQF
	`) + "\n"
	if got := read(t, path); got != want {
		t.Errorf("document =\n%s\nwant\n%s", got, want)
	}
}

// TestCommitRemovesEmptiedBlocks verifies emptied groups and top-level blocks
// are removed rather than left as empty mappings.
func TestCommitRemovesEmptiedBlocks(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, sampleDocument)

	for _, name := range []string{"api", "web"} {
		snap, err := repo.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Commit(snap, secrets.Group{Name: name}); err != nil {
			t.Fatalf("Commit %s: %v", name, err)
		}
	}
	if got := strings.TrimSpace(read(t, path)); got != "# managed by envx\n{}" &&
		got != "{}" {
		t.Errorf("document = %q, want empty mapping", got)
	}
	list, err := repo.List()
	if err != nil || len(list) != 0 {
		t.Errorf("List = %+v, %v", list, err)
	}
}

// TestCommitCreatesDocument verifies a first commit writes public keys before
// secrets and new keys in sorted order.
func TestCommitCreatesDocument(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, "")

	err := repo.Commit(secrets.Snapshot{}, secrets.Group{
		Name: "api", PublicKey: "pub", Values: map[string]string{"B": "2", "A": "1"},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	want := str.Dedent(`
		public-keys:
		  api: pub
		secrets:
		  api:
		    A: "1"
		    B: "2"
	`) + "\n"
	if got := read(t, path); got != want {
		t.Errorf("document =\n%s\nwant\n%s", got, want)
	}
}

// TestCommitRejectsInvalidValues verifies a malformed replacement is rejected
// without touching the document.
func TestCommitRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	repo, path := newRepository(t, sampleDocument)
	snap, err := repo.Read("api")
	if err != nil {
		t.Fatal(err)
	}

	err = repo.Commit(snap, secrets.Group{
		Name: "api", Values: map[string]string{"K": "encrypted-age:"},
	})
	if err == nil {
		t.Fatal("Commit with malformed envelope succeeded")
	}
	if read(t, path) != sampleDocument {
		t.Error("document changed after rejected commit")
	}
}

// TestParseRejectsInvalidDocuments verifies strict decoding and structural
// validation reject malformed documents.
func TestParseRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"unknown field":   "secret:\n  api: {}\n",
		"duplicate group": "secrets:\n  api: {A: x}\n  API: {B: y}\n",
		"bad envelope":    "secrets:\n  api:\n    A: encrypted-age:\n",
		"empty public":    "public-keys:\n  api: \"\"\n",
		"nested value":    "secrets:\n  api:\n    A: {x: y}\n",
	}
	for name, content := range tests {
		repo, _ := newRepository(t, content)
		if _, err := repo.List(); err == nil {
			t.Errorf("%s: List succeeded, want error", name)
		}
	}
}
