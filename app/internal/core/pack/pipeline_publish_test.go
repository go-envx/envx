package pack

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

// TestSwapPathRestoresPrevious verifies a failed forced rename moves the
// previous output back into place.
func TestSwapPathRestoresPrevious(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "dist")
	writeFile(t, target, "envx.yaml", "previous\n")

	sc := scratch{
		staged:   filepath.Join(dir, "missing"),
		previous: filepath.Join(dir, "previous"),
	}
	if err := sc.swapPath(target, true); err == nil {
		t.Fatal("swapPath succeeded, want an error")
	}
	if got := readFile(t, target, "envx.yaml"); got != "previous\n" {
		t.Errorf("previous output = %q, want restored", got)
	}
	absent(t, dir, "previous")
}

// TestSwapContentsRollsBack verifies a failure partway through moving staged
// entries removes those already moved and leaves existing entries intact.
func TestSwapContentsRollsBack(t *testing.T) {
	t.Parallel()
	scratchDir := t.TempDir()
	staged := filepath.Join(scratchDir, "bundle")
	writeFile(t, staged, "a.yaml", "new\n")
	writeFile(t, staged, "b/b.yaml", "new\n")
	dir := t.TempDir()
	writeFile(t, dir, "b/existing.yaml", "keep\n")

	sc := scratch{staged: staged, previous: filepath.Join(scratchDir, "previous")}
	err := sc.swapContents(dir, false)
	if !errors.Is(err, ErrOutputNotEmpty) {
		t.Fatalf("err = %v, want ErrOutputNotEmpty", err)
	}
	if got := listFiles(t, dir); !slices.Equal(got, []string{"b/existing.yaml"}) {
		t.Errorf("destination files = %v, want only the existing entry", got)
	}
	if got := listFiles(t, staged); !slices.Equal(got, []string{"a.yaml", "b/b.yaml"}) {
		t.Errorf("staged files = %v, want both moved back", got)
	}
}
