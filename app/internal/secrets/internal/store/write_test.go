package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWrites verifies additions, updates, deletion, and retention of an
// empty group after its last secret is removed.
func TestWrites(t *testing.T) {
	t.Parallel()

	path := writeDocument(t, "secrets:\n  production:\n    first: one\n    second: two\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := document.SetPublicKey("production", "age-public-key:age1prod"); err != nil {
		t.Fatalf("SetPublicKey() add error = %v", err)
	}
	if err := document.SetPublicKey(
		"PRODUCTION", "age-public-key:age1updated",
	); err != nil {
		t.Fatalf("SetPublicKey() update error = %v", err)
	}
	if err := document.SetSecret("PRODUCTION", "first", "updated"); err != nil {
		t.Fatalf("SetSecret() update error = %v", err)
	}
	if err := document.SetSecret("shared", "token", "value"); err != nil {
		t.Fatalf("SetSecret() group add error = %v", err)
	}
	if err := document.SetSecret("production", "third", "three"); err != nil {
		t.Fatalf("SetSecret() entry add error = %v", err)
	}
	if deleted, err := document.DeleteSecret(
		"production", "second",
	); err != nil || !deleted {
		t.Fatalf("DeleteSecret() = %v, %v; want true, nil", deleted, err)
	}
	if deleted, err := document.DeleteSecret(
		"production", "missing",
	); err != nil || deleted {
		t.Fatalf("DeleteSecret() missing = %v, %v; want false, nil", deleted, err)
	}

	if err := document.Save(0); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := Open(path)
	if err != nil {
		t.Fatalf("Open() after Save error = %v", err)
	}
	if got, ok := reloaded.PublicKey("production"); !ok ||
		got != "age-public-key:age1updated" {
		t.Errorf("PublicKey() after Save = %q, %v", got, ok)
	}
	if got, ok := reloaded.Secret("production", "first"); !ok || got.Value != "updated" {
		t.Errorf("Secret() after Save = %#v, %v", got, ok)
	}
	if _, ok := reloaded.Secret("production", "second"); ok {
		t.Error("deleted secret is still present")
	}
	if _, ok := reloaded.Secret("shared", "token"); !ok {
		t.Error("new group secret is absent")
	}
}

// TestCommentAndOrderPreservation verifies node mutation keeps comments and
// leaves existing entries in their original order.
func TestCommentAndOrderPreservation(t *testing.T) {
	t.Parallel()

	path := writeDocument(t, "# workspace secrets\n"+
		"secrets:\n"+
		"  production:\n"+
		"    # keep this comment\n"+
		"    first: one\n"+
		"    second: two\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := document.SetSecret("production", "first", "updated"); err != nil {
		t.Fatalf("SetSecret() update error = %v", err)
	}
	if err := document.SetSecret("production", "third", "three"); err != nil {
		t.Fatalf("SetSecret() add error = %v", err)
	}
	if err := document.Save(0); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	//nolint:gosec // G304: path is created inside this test's temporary directory.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	if !strings.Contains(output, "# keep this comment") {
		t.Fatalf("saved document lost comment:\n%s", output)
	}
	first := strings.Index(output, "first: updated")
	second := strings.Index(output, "second: two")
	third := strings.Index(output, "third: three")
	if first < 0 || second < 0 || third < 0 || first > second || second > third {
		t.Fatalf("saved document order is wrong:\n%s", output)
	}
}

// TestSavePreservesTwoSpaceIndent verifies an existing 2-space document keeps
// its indentation width on save.
func TestSavePreservesTwoSpaceIndent(t *testing.T) {
	t.Parallel()

	path := writeDocument(t, "secrets:\n  production:\n    first: one\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := document.SetSecret("production", "second", "two"); err != nil {
		t.Fatalf("SetSecret() error = %v", err)
	}
	if err := document.Save(0); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertIndentStep(t, path, 2)
}

// TestSavePreservesFourSpaceIndent verifies an existing 4-space document keeps
// its indentation width on save despite the fallback and default being 2.
func TestSavePreservesFourSpaceIndent(t *testing.T) {
	t.Parallel()

	path := writeDocument(t, "secrets:\n    production:\n        first: one\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := document.SetSecret("production", "second", "two"); err != nil {
		t.Fatalf("SetSecret() error = %v", err)
	}
	if err := document.Save(0); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertIndentStep(t, path, 4)
}

// TestSaveUsesProvidedDefaultIndent verifies a document with no detectable
// indentation of its own is written using the caller's provided default.
func TestSaveUsesProvidedDefaultIndent(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name          string
		defaultIndent int
		want          int
	}{
		{name: "two spaces", defaultIndent: 2, want: 2},
		{name: "four spaces", defaultIndent: 4, want: 4},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "secrets.yaml")
			document, err := Open(path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if err := document.SetSecret("production", "first", "one"); err != nil {
				t.Fatalf("SetSecret() error = %v", err)
			}
			if err := document.Save(testCase.defaultIndent); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			assertIndentStep(t, path, testCase.want)
		})
	}
}

// assertIndentStep checks the group and key lines are indented one and two steps.
func assertIndentStep(t *testing.T, path string, step int) {
	t.Helper()
	if got := leadingSpaces(t, path, "production:"); got != step {
		t.Errorf("group indent = %d, want %d", got, step)
	}
	if got := leadingSpaces(t, path, "first:"); got != 2*step {
		t.Errorf("key indent = %d, want %d", got, 2*step)
	}
}

// leadingSpaces returns the number of leading spaces on the first saved line
// containing needle.
func leadingSpaces(t *testing.T, path, needle string) int {
	t.Helper()
	//nolint:gosec // G304: path is created inside this test's temporary directory.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, needle) {
			return len(line) - len(strings.TrimLeft(line, " "))
		}
	}
	t.Fatalf("line %q not found in:\n%s", needle, data)
	return 0
}
