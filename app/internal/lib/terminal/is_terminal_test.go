package terminal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/terminal"
)

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	if terminal.IsTerminal(nil) {
		t.Error("IsTerminal(nil) = true, want false")
	}

	tmpFile, err := os.Create(filepath.Join(t.TempDir(), "testfile"))
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	t.Cleanup(func() {
		_ = tmpFile.Close()
	})

	if terminal.IsTerminal(tmpFile) {
		t.Error("IsTerminal(regular file) = true, want false")
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(): %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	if terminal.IsTerminal(r) {
		t.Error("IsTerminal(pipe reader) = true, want false")
	}
	if terminal.IsTerminal(w) {
		t.Error("IsTerminal(pipe writer) = true, want false")
	}
}
