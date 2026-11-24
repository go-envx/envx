package atomicfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/atomicfile"
)

func TestLockUnlock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "lock_test.txt")
	lockFilePath := filepath.Join(dir, ".test.lock")

	fl, err := atomicfile.Lock(lockFilePath)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if _, err := os.Stat(lockFilePath); err != nil {
		t.Errorf("lock file missing: %v", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("Lock created an unrelated target file")
	}

	if err := fl.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Double unlock is safe
	if err := fl.Unlock(); err != nil {
		t.Errorf("double Unlock error: %v", err)
	}
}
