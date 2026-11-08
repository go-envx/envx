package atomicfile_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/atomicfile"
)

func TestWriteCreates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "out.yaml")
	want := []byte("key: value\n")

	if err := atomicfile.Write(target, want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	//nolint:gosec // test reads back temp target
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("contents = %q, want %q", got, want)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("permissions = %o, want 644", perm)
	}
}

func TestWriteOverwritesPreservingPermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "out.yaml")

	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o640); err != nil { //nolint:gosec // test mode
		t.Fatal(err)
	}

	if err := atomicfile.Write(target, []byte("new")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	//nolint:gosec // test reads back temp target
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o640 {
		t.Errorf("permissions = %o, want 640", perm)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, found %d (temp leak?)", len(entries))
	}
}

func TestWritePrivateCreates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "secure.dat")

	if err := atomicfile.WritePrivate(target, []byte("private")); err != nil {
		t.Fatalf("WritePrivate: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}

func TestWritePrivateOverwrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "secure.dat")

	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o644); err != nil { //nolint:gosec // test mode
		t.Fatal(err)
	}

	if err := atomicfile.WritePrivate(target, []byte("new")); err != nil {
		t.Fatalf("WritePrivate: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}

func TestWriteAtomicAliases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "alias.dat")

	if err := atomicfile.WriteAtomic(target, []byte("public")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("permissions = %o, want 644", perm)
	}

	privateTarget := filepath.Join(dir, "private_alias.dat")
	if err := atomicfile.WriteAtomicPrivate(privateTarget, []byte("priv")); err != nil {
		t.Fatalf("WriteAtomicPrivate: %v", err)
	}
	infoPriv, err := os.Stat(privateTarget)
	if err != nil {
		t.Fatal(err)
	}
	if perm := infoPriv.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}
