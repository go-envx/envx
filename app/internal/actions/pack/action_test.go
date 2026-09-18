package pack

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeWorkspace scaffolds a minimal single-project workspace in a fresh temp dir
// and returns the manifest path.
func writeWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"envx.yaml": "environments: [development, production]\n" +
			"projects:\n  app:\n    includes: [env/app]\n",
		"env/app.yaml":             "A: base\n",
		"env/app.development.yaml": "A: dev\n",
		"env/app.production.yaml":  "A: prod\n",
	}
	for path, body := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "envx.yaml")
}

// runCmd wires a pack command under a root that owns the persistent --config
// flag, executes it with args, and returns captured stdout and any error.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "envx"}
	root.PersistentFlags().String("config", "", "path to envx.yaml")
	root.AddCommand(NewCommand())
	stdout := new(bytes.Buffer)
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs(append([]string{"pack"}, args...))
	err := root.Execute()
	return stdout.String(), err
}

// TestPackCommandWritesBundle verifies the command copies the selected
// environment's files and reports them.
func TestPackCommandWritesBundle(t *testing.T) {
	t.Parallel()
	cfg := writeWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	stdout, err := runCmd(t, "--config", cfg, "-e", "production", "--out", out)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if !strings.Contains(stdout, "app.production.yaml") {
		t.Errorf("summary missing packed file, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(out, "envx.yaml")); err != nil {
		t.Errorf("manifest not written: %v", err)
	}
	dropped := filepath.Join(out, "app.development.yaml")
	if _, err := os.Stat(dropped); !os.IsNotExist(err) {
		t.Error("unselected development overlay was copied")
	}
}

// seedDir creates dir and writes one stale file into it, returning the stale
// file's path so a test can assert whether it survived.
func seedDir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return stale
}

// TestPackCommandRefusesNonEmptyOut verifies the command refuses a non-empty
// output directory by default.
func TestPackCommandRefusesNonEmptyOut(t *testing.T) {
	t.Parallel()
	cfg := writeWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	seedDir(t, out)

	_, err := runCmd(t, "--config", cfg, "-e", "production", "--out", out)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("err = %v, want a non-empty-directory error", err)
	}
}

// TestPackCommandForceReplacesOut verifies --force lets the command replace a
// non-empty output directory.
func TestPackCommandForceReplacesOut(t *testing.T) {
	t.Parallel()
	cfg := writeWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	stale := seedDir(t, out)

	_, err := runCmd(t, "--config", cfg, "-e", "production", "--out", out, "--force")
	if err != nil {
		t.Fatalf("pack --force: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("--force did not clear the stale file")
	}
	if _, err := os.Stat(filepath.Join(out, "envx.yaml")); err != nil {
		t.Errorf("manifest not written: %v", err)
	}
}

// TestPackCommandRequiresOut verifies --out is a required flag.
func TestPackCommandRequiresOut(t *testing.T) {
	t.Parallel()
	cfg := writeWorkspace(t)

	if _, err := runCmd(t, "--config", cfg, "-e", "production"); err == nil ||
		!strings.Contains(err.Error(), "out") {
		t.Fatalf("err = %v, want a required --out error", err)
	}
}
