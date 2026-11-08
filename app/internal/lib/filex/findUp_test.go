package filex

import (
	"os"
	"path/filepath"
	"testing"
)

// mkdirAll creates a nested directory under root and returns its absolute path.
func mkdirAll(t *testing.T, root string, parts ...string) string {
	t.Helper()
	dir := filepath.Join(append([]string{root}, parts...)...)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// touch writes an empty file at the join of dir and name.
func touch(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFindUpInStartDir verifies a file present in the start directory is
// returned.
func TestFindUpInStartDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := touch(t, dir, "target.yaml")

	got, err := FindUp(dir, "target.yaml")
	if err != nil {
		t.Fatalf("FindUp: %v", err)
	}
	if got != want {
		t.Errorf("FindUp() = %q, want %q", got, want)
	}
}

// TestFindUpInAncestor verifies the search ascends parents to find the file.
func TestFindUpInAncestor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	want := touch(t, root, "target.yaml")
	start := mkdirAll(t, root, "a", "b", "c")

	got, err := FindUp(start, "target.yaml")
	if err != nil {
		t.Fatalf("FindUp: %v", err)
	}
	if got != want {
		t.Errorf("FindUp() = %q, want %q", got, want)
	}
}

// TestFindUpReturnsAbsolute verifies a relative start directory yields an
// absolute result path.
func TestFindUpReturnsAbsolute(t *testing.T) {
	// t.Chdir forbids t.Parallel.
	dir := t.TempDir()
	touch(t, dir, "target.yaml")
	t.Chdir(dir)

	got, err := FindUp(".", "target.yaml")
	if err != nil {
		t.Fatalf("FindUp: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}
