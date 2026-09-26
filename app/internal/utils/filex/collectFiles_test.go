package filex_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/go-envx/envx/app/internal/utils/filex"
)

// TestCollectFiles verifies recursive collection of files in sorted order.
func TestCollectFiles(t *testing.T) {
	t.Parallel()

	mockFS := fstest.MapFS{
		"root.txt":             {Data: []byte("root")},
		"nested/sub/child.txt": {Data: []byte("child")},
		"nested/alpha.txt":     {Data: []byte("alpha")},
	}

	files, err := filex.CollectFiles(mockFS)
	if err != nil {
		t.Fatalf("CollectFiles failed: %v", err)
	}

	want := []string{
		"nested/alpha.txt",
		"nested/sub/child.txt",
		"root.txt",
	}

	if !reflect.DeepEqual(files, want) {
		t.Errorf("CollectFiles() = %v, want %v", files, want)
	}
}

// TestCollectFilesEmpty verifies an empty fs returns an empty slice without error.
func TestCollectFilesEmpty(t *testing.T) {
	t.Parallel()

	mockFS := fstest.MapFS{}
	files, err := filex.CollectFiles(mockFS)
	if err != nil {
		t.Fatalf("CollectFiles on empty FS failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}
