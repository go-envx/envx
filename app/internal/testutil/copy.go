package testutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CopyDir copies the fixture directory at src into a new isolated temporary
// directory created via tb.TempDir() and returns the path to the copied directory.
//
// The copied directory tree is writable so mutation tests can safely modify,
// create, and delete files without affecting the source fixtures.
func CopyDir(tb testing.TB, src string) string {
	tb.Helper()
	dst := tb.TempDir()
	CopyDirTo(tb, src, dst)
	return dst
}

// CopyDirTo copies the fixture directory at src into dst.
// If dst does not exist, it is created. It fails tb immediately on error.
func CopyDirTo(tb testing.TB, src, dst string) {
	tb.Helper()
	if err := CopyTree(src, dst); err != nil {
		tb.Fatalf("testutil: copy directory %s to %s: %v", src, dst, err)
	}
}

// CopyFile copies the source file at src into a new isolated temporary directory
// created via tb.TempDir() and returns the path to the copied file.
func CopyFile(tb testing.TB, src string) string {
	tb.Helper()
	dstDir := tb.TempDir()
	dst := filepath.Join(dstDir, filepath.Base(src))
	CopyFileTo(tb, src, dst)
	return dst
}

// CopyFileTo copies the source file at src to dst. It fails tb immediately on error.
func CopyFileTo(tb testing.TB, src, dst string) {
	tb.Helper()
	if err := CopyFileContents(src, dst); err != nil {
		tb.Fatalf("testutil: copy file %s to %s: %v", src, dst, err)
	}
}

// CopyTree recursively copies the directory tree at src into dst.
// Existing files in dst are overwritten. All created directories and files
// are granted owner read and write permissions so mutation tests can modify them.
// Relative symlinks are replicated with their relative target intact.
func CopyTree(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source %s: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %s is not a directory", src)
	}

	dstInfo, err := os.Stat(dst)
	if err == nil && !dstInfo.IsDir() {
		return fmt.Errorf("destination %s exists and is not a directory", dst)
	}

	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("resolve source path %s: %w", src, err)
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("resolve destination path %s: %w", dst, err)
	}

	rel, err := filepath.Rel(absSrc, absDst)
	sep := string(filepath.Separator)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+sep) {
		return fmt.Errorf("destination %s is inside source %s", dst, src)
	}

	dirPerm := srcInfo.Mode().Perm() | 0o700
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("create destination directory %s: %w", dst, err)
	}
	_ = os.Chmod(dst, dirPerm)

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("relpath %s from %s: %w", path, src, err)
		}
		target := filepath.Join(dst, relPath)

		if d.Type()&fs.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read symlink %s: %w", path, err)
			}
			_ = os.Remove(target)
			//nolint:gosec // G122: test fixture symlink replication
			if err := os.Symlink(linkTarget, target); err != nil {
				return fmt.Errorf("create symlink %s: %w", target, err)
			}
			return nil
		}

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("stat dir %s: %w", path, err)
			}
			perm := info.Mode().Perm() | 0o700
			if err := os.MkdirAll(target, perm); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}
			if err := os.Chmod(target, perm); err != nil {
				return fmt.Errorf("chmod dir %s: %w", target, err)
			}
			return nil
		}

		if d.Type().IsRegular() {
			return copyFile(path, target)
		}

		return fmt.Errorf("unsupported file type %s: %s", d.Type(), path)
	})
}

// CopyFileContents copies a single file from src to dst, preserving permissions
// while ensuring owner read and write permissions.
func CopyFileContents(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source file %s: %w", src, err)
	}
	if info.IsDir() {
		return fmt.Errorf("source %s is a directory, not a file", src)
	}

	dstInfo, err := os.Stat(dst)
	if err == nil && os.SameFile(info, dstInfo) {
		return fmt.Errorf("source %s and destination %s are the same file", src, dst)
	}

	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	//nolint:gosec // G304: test utility operates on fixture paths
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source %s: %w", src, err)
	}

	perm := info.Mode().Perm() | 0o600

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", dst, err)
	}

	// Remove destination first to ensure existing read-only files can be overwritten.
	_ = os.Remove(dst)

	//nolint:gosec // G304: test utility destination path
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("create destination %s: %w", dst, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}

	if err := dstFile.Chmod(perm); err != nil {
		_ = dstFile.Close()
		return fmt.Errorf("chmod %s: %w", dst, err)
	}

	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("close destination %s: %w", dst, err)
	}

	return nil
}
