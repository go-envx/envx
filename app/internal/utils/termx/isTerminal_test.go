package termx_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/utils/termx"
)

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	if termx.IsTerminal("not a file") {
		t.Error("IsTerminal(string) = true, want false")
	}
	if termx.IsTerminal(new(bytes.Buffer)) {
		t.Error("IsTerminal(*bytes.Buffer) = true, want false")
	}

	// Regular files are not terminals
	tmpFile, err := os.Create(filepath.Join(t.TempDir(), "testfile"))
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	t.Cleanup(func() {
		_ = tmpFile.Close()
	})

	if termx.IsTerminal(tmpFile) {
		t.Error("IsTerminal(tmpFile) = true, want false")
	}

	// Pipes are not terminals
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(): %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	if termx.IsTerminal(r) {
		t.Error("IsTerminal(pipe reader) = true, want false")
	}
	if termx.IsTerminal(w) {
		t.Error("IsTerminal(pipe writer) = true, want false")
	}
}
