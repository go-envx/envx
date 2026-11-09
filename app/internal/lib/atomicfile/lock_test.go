package atomicfile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestReadModifyWriteConcurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "counter.txt")
	lockPath := filepath.Join(dir, ".test.lock")

	// Initialize file with 0
	if err := atomicfile.Write(target, []byte("0")); err != nil {
		t.Fatal(err)
	}

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)

	increment := func(data []byte) ([]byte, error) {
		var count int
		if _, err := fmt.Sscanf(string(data), "%d", &count); err != nil {
			return nil, err
		}
		return fmt.Appendf(nil, "%d", count+1), nil
	}
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			err := atomicfile.ReadModifyWrite(lockPath, target, increment)
			if err != nil {
				t.Errorf("ReadModifyWrite error: %v", err)
			}
		}()
	}

	wg.Wait()

	//nolint:gosec // test reads back temp target
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading counter file: %v", err)
	}

	var finalCount int
	if _, err := fmt.Sscanf(string(data), "%d", &finalCount); err != nil {
		t.Fatalf("parsing final count %q: %v", string(data), err)
	}

	if finalCount != concurrency {
		t.Errorf("final count = %d, want %d", finalCount, concurrency)
	}
}

func TestReadModifyWriteNewFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "new_file.txt")
	lockPath := filepath.Join(dir, ".test.lock")

	err := atomicfile.ReadModifyWrite(lockPath, target, func(data []byte) ([]byte, error) {
		if len(data) != 0 {
			return nil, fmt.Errorf("expected empty data, got %q", string(data))
		}
		return []byte("created"), nil
	})
	if err != nil {
		t.Fatalf("ReadModifyWrite on new file: %v", err)
	}

	//nolint:gosec // test reads back temp target
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "created" {
		t.Errorf("got %q, want 'created'", string(data))
	}
}
