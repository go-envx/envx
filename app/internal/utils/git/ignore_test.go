package git

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureIgnoredRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if err := EnsureIgnored(""); err == nil {
		t.Fatal("EnsureIgnored() accepted an empty path")
	}
	if err := EnsureIgnored("   "); err == nil {
		t.Fatal("EnsureIgnored() accepted a whitespace path")
	}
}

func TestEnsureIgnoredCreatesLocalRule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "build", "cache.dat")

	if err := EnsureIgnored(targetPath); err != nil {
		t.Fatalf("EnsureIgnored(): %v", err)
	}

	ignoreFile := filepath.Join(dir, "build", ".gitignore")
	data, err := os.ReadFile(ignoreFile) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatalf("ReadFile(.gitignore): %v", err)
	}
	if !strings.Contains(string(data), "cache.dat\n") {
		t.Errorf(".gitignore = %q, want cache.dat rule", string(data))
	}

	// Calling again should be idempotent and not duplicate the entry.
	if err := EnsureIgnored(targetPath); err != nil {
		t.Fatalf("EnsureIgnored() second call: %v", err)
	}
	data2, err := os.ReadFile(ignoreFile) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatalf("ReadFile(.gitignore): %v", err)
	}
	if !bytes.Equal(data2, data) {
		t.Errorf("second call modified .gitignore: %q vs %q", string(data2), string(data))
	}
}

func TestEnsureIgnoredSkipsWhenGitUnavailable(t *testing.T) {
	gitlessPath := t.TempDir()
	t.Setenv("PATH", gitlessPath)

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "cache.dat")
	if err := EnsureIgnored(targetPath); err != nil {
		t.Fatalf("EnsureIgnored(): %v", err)
	}

	// Should not have created a .gitignore when git is missing.
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Error("expected .gitignore not to exist when git is unavailable")
	}
}

func TestHasIgnoreRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		target  string
		want    bool
	}{
		{
			name:    "exact match",
			content: "# header\ncache.dat\n",
			target:  "cache.dat",
			want:    true,
		},
		{
			name:    "leading slash match",
			content: "/cache.dat\n",
			target:  "cache.dat",
			want:    true,
		},
		{
			name:    "no match",
			content: "other.dat\n",
			target:  "cache.dat",
			want:    false,
		},
		{
			name:    "substring is not match",
			content: "my-cache.dat\n",
			target:  "cache.dat",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasIgnoreRule(tt.content, tt.target)
			if got != tt.want {
				t.Errorf("hasIgnoreRule() = %v, want %v", got, tt.want)
			}
		})
	}
}
