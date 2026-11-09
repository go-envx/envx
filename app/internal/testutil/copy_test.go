package testutil_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/testutil"
)

func writeTestFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	//nolint:gosec // test helper with test-local paths
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	//nolint:gosec // test helper with test-local paths
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func mkdirTestDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestCopyDir(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	subDir := filepath.Join(src, "sub")
	emptyDir := filepath.Join(src, "empty")
	mkdirTestDir(t, subDir)
	mkdirTestDir(t, emptyDir)

	file1 := filepath.Join(src, "root.txt")
	writeTestFile(t, file1, []byte("root content"), 0o600)

	file2 := filepath.Join(subDir, "nested.yaml")
	writeTestFile(t, file2, []byte("key: value\n"), 0o600)

	dst := testutil.CopyDir(t, src)
	if dst == src {
		t.Fatalf("expected dst to be separate directory, got %s", dst)
	}

	// Verify all contents exist with expected data.
	gotRoot := readTestFile(t, filepath.Join(dst, "root.txt"))
	if string(gotRoot) != "root content" {
		t.Errorf("dst root.txt content = %q, want %q", string(gotRoot), "root content")
	}

	gotNested := readTestFile(t, filepath.Join(dst, "sub", "nested.yaml"))
	if string(gotNested) != "key: value\n" {
		t.Errorf(
			"dst nested.yaml content = %q, want %q",
			string(gotNested),
			"key: value\n",
		)
	}

	emptyInfo, err := os.Stat(filepath.Join(dst, "empty"))
	if err != nil {
		t.Fatalf("stat dst empty dir: %v", err)
	}
	if !emptyInfo.IsDir() {
		t.Errorf("expected dst empty to be directory")
	}

	// Mutation test: mutate destination and confirm source is unmodified.
	dstRootPath := filepath.Join(dst, "root.txt")
	writeTestFile(t, dstRootPath, []byte("mutated root"), 0o600)

	srcRoot := readTestFile(t, file1)
	if string(srcRoot) != "root content" {
		t.Errorf(
			"src root.txt mutated: got %q, want %q",
			string(srcRoot),
			"root content",
		)
	}

	// Create new file in dst; confirm src does not have it.
	newDstFile := filepath.Join(dst, "brand_new.txt")
	writeTestFile(t, newDstFile, []byte("brand new"), 0o600)
	if _, err := os.Stat(filepath.Join(src, "brand_new.txt")); !os.IsNotExist(err) {
		t.Errorf("brand_new.txt leaked into source directory")
	}

	// Delete file in dst; confirm src still has it.
	if err := os.Remove(filepath.Join(dst, "sub", "nested.yaml")); err != nil {
		t.Fatalf("remove nested.yaml from dst: %v", err)
	}
	if _, err := os.Stat(file2); err != nil {
		t.Errorf("src nested.yaml missing after dst removal: %v", err)
	}
}

func TestCopyDirTo(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "data.txt"), []byte("hello"), 0o600)

	dst := filepath.Join(t.TempDir(), "target")
	testutil.CopyDirTo(t, src, dst)

	got := readTestFile(t, filepath.Join(dst, "data.txt"))
	if string(got) != "hello" {
		t.Errorf("got %q, want hello", string(got))
	}
}

func TestCopyDir_EmptyDirectory(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := testutil.CopyDir(t, src)

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be directory", dst)
	}

	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatalf("read dir dst: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries in empty copy, got %d", len(entries))
	}
}

func TestCopyDir_Symlink(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	targetFile := filepath.Join(src, "target.txt")
	writeTestFile(t, targetFile, []byte("target data"), 0o600)

	symlinkPath := filepath.Join(src, "link.txt")
	if err := os.Symlink("target.txt", symlinkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	dst := testutil.CopyDir(t, src)

	dstLink := filepath.Join(dst, "link.txt")
	linkInfo, err := os.Lstat(dstLink)
	if err != nil {
		t.Fatalf("lstat dst link: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be symlink", dstLink)
	}

	linkTarget, err := os.Readlink(dstLink)
	if err != nil {
		t.Fatalf("readlink dst link: %v", err)
	}
	if linkTarget != "target.txt" {
		t.Errorf("symlink target = %q, want %q", linkTarget, "target.txt")
	}

	content := readTestFile(t, dstLink)
	if string(content) != "target data" {
		t.Errorf("read through link = %q, want %q", string(content), "target data")
	}
}

func TestCopyDir_EnsuresWritablePermissions(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	readOnlyFile := filepath.Join(src, "readonly.txt")
	writeTestFile(t, readOnlyFile, []byte("readonly"), 0o400)

	dst := testutil.CopyDir(t, src)

	dstFile := filepath.Join(dst, "readonly.txt")
	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("stat dst file: %v", err)
	}
	if info.Mode().Perm()&0o200 == 0 {
		t.Errorf(
			"expected dst file to have owner write permission, got %o",
			info.Mode().Perm(),
		)
	}

	// Confirm destination file can be opened and modified.
	writeTestFile(t, dstFile, []byte("updated"), 0o600)
}

func TestCopyTree_OverwriteExistingReadOnlyFile(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "file.txt"), []byte("new data"), 0o600)

	dst := t.TempDir()
	dstFile := filepath.Join(dst, "file.txt")
	writeTestFile(t, dstFile, []byte("old read-only"), 0o400)

	if err := testutil.CopyTree(src, dst); err != nil {
		t.Fatalf("CopyTree failed to overwrite read-only file: %v", err)
	}

	got := readTestFile(t, dstFile)
	if string(got) != "new data" {
		t.Errorf("got %q, want %q", string(got), "new data")
	}
}

func TestCopyTree_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T) (src, dst string)
		wantMatch string
	}{
		{
			name: "non-existent source",
			setup: func(t *testing.T) (string, string) {
				return "/non/existent/source/path", t.TempDir()
			},
			wantMatch: "stat source",
		},
		{
			name: "source is file",
			setup: func(t *testing.T) (string, string) {
				f := filepath.Join(t.TempDir(), "file.txt")
				writeTestFile(t, f, []byte("data"), 0o600)
				return f, t.TempDir()
			},
			wantMatch: "not a directory",
		},
		{
			name: "destination is file",
			setup: func(t *testing.T) (string, string) {
				f := filepath.Join(t.TempDir(), "dst_file.txt")
				writeTestFile(t, f, []byte("data"), 0o600)
				return t.TempDir(), f
			},
			wantMatch: "exists and is not a directory",
		},
		{
			name: "destination inside source",
			setup: func(t *testing.T) (string, string) {
				src := t.TempDir()
				return src, filepath.Join(src, "sub", "inside")
			},
			wantMatch: "is inside source",
		},
		{
			name: "destination is source itself",
			setup: func(t *testing.T) (string, string) {
				src := t.TempDir()
				return src, src
			},
			wantMatch: "is inside source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src, dst := tt.setup(t)
			err := testutil.CopyTree(src, dst)
			if err == nil {
				t.Fatalf("expected error matching %q, got nil", tt.wantMatch)
			}
			if !strings.Contains(err.Error(), tt.wantMatch) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantMatch)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "hello.txt")
	writeTestFile(t, srcFile, []byte("world"), 0o600)

	copiedFile := testutil.CopyFile(t, srcFile)
	if filepath.Base(copiedFile) != "hello.txt" {
		t.Errorf("expected base name hello.txt, got %s", filepath.Base(copiedFile))
	}

	content := readTestFile(t, copiedFile)
	if string(content) != "world" {
		t.Errorf("got %q, want world", string(content))
	}

	// Verify mutating copied file does not alter source file.
	writeTestFile(t, copiedFile, []byte("mutated"), 0o600)
	srcContent := readTestFile(t, srcFile)
	if string(srcContent) != "world" {
		t.Errorf("src altered: got %q, want world", string(srcContent))
	}
}

func TestCopyFileTo(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "origin.txt")
	writeTestFile(t, srcFile, []byte("origin content"), 0o600)

	targetDir := t.TempDir()
	dstFile := filepath.Join(targetDir, "nested", "dest.txt")

	testutil.CopyFileTo(t, srcFile, dstFile)

	content := readTestFile(t, dstFile)
	if string(content) != "origin content" {
		t.Errorf("got %q, want origin content", string(content))
	}
}

func TestCopyFileContents_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T) (src, dst string)
		wantMatch string
	}{
		{
			name: "source does not exist",
			setup: func(t *testing.T) (string, string) {
				return "/non/existent/file.txt", filepath.Join(t.TempDir(), "dst.txt")
			},
			wantMatch: "stat source file",
		},
		{
			name: "source is directory",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), filepath.Join(t.TempDir(), "dst.txt")
			},
			wantMatch: "is a directory",
		},
		{
			name: "source and destination are same file",
			setup: func(t *testing.T) (string, string) {
				f := filepath.Join(t.TempDir(), "same.txt")
				writeTestFile(t, f, []byte("content"), 0o600)
				return f, f
			},
			wantMatch: "same file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src, dst := tt.setup(t)
			err := testutil.CopyFileContents(src, dst)
			if err == nil {
				t.Fatalf("expected error matching %q, got nil", tt.wantMatch)
			}
			if !strings.Contains(err.Error(), tt.wantMatch) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantMatch)
			}
		})
	}
}

type fakeTB struct {
	testing.TB
	failed   bool
	fatalMsg string
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) TempDir() string {
	d, err := os.MkdirTemp("", "faketb-*")
	if err != nil {
		panic(err)
	}
	return d
}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.failed = true
	f.fatalMsg = fmt.Sprintf(format, args...)
}

func TestFakeTB_Failures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		run       func(tb testing.TB)
		wantMatch string
	}{
		{
			name: "CopyDirTo fatal on invalid source",
			run: func(tb testing.TB) {
				testutil.CopyDirTo(tb, "/non/existent/path", tb.TempDir())
			},
			wantMatch: "testutil: copy directory",
		},
		{
			name: "CopyFileTo fatal on invalid source",
			run: func(tb testing.TB) {
				testutil.CopyFileTo(
					tb,
					"/non/existent/file.txt",
					filepath.Join(tb.TempDir(), "out.txt"),
				)
			},
			wantMatch: "testutil: copy file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeTB{}
			tt.run(fake)
			if !fake.failed {
				t.Fatal("expected fakeTB to fail")
			}
			if !strings.Contains(fake.fatalMsg, tt.wantMatch) {
				t.Errorf("fatal %q does not contain %q", fake.fatalMsg, tt.wantMatch)
			}
		})
	}
}
