package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// mockPackFactory implements Factory for unit testing.
type mockPackFactory struct {
	err error
}

func (m *mockPackFactory) PackService(configPath string) (*pack.Service, error) {
	if m.err != nil {
		return nil, m.err
	}
	app, err := core.NewAppFactory()
	if err != nil {
		return nil, err
	}
	return app.PackService(configPath)
}

// executePack builds the pack command, executes it with args, and returns its
// captured stdout.
func executePack(
	t *testing.T, factory Factory, configPath string, args ...string,
) (stdout string, err error) {
	t.Helper()

	cmd := NewPackCommand(factory)
	flags.Bind(cmd.PersistentFlags(), &flags.Config)
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", configPath}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), err
}

// writePackWorkspace scaffolds a minimal single-project workspace in a fresh temp
// dir and returns the manifest path.
func writePackWorkspace(t *testing.T) string {
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

// seedPackDir creates dir and writes one stale file into it, returning the stale
// file's path so a test can assert whether it survived.
func seedPackDir(t *testing.T, dir string) string {
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

// TestPackCommandWritesBundle verifies the command copies the selected
// environment's files and reports them.
func TestPackCommandWritesBundle(t *testing.T) {
	t.Parallel()

	cfg := writePackWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	stdout, err := executePack(
		t, &mockPackFactory{}, cfg, "-e", "production", "--out", out,
	)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if !strings.Contains(stdout, "app.production.yaml") {
		t.Errorf("summary missing packed file, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(out, "envx.yaml")); err != nil {
		t.Errorf("manifest not written: %v", err)
	}
	dropped := filepath.Join(out, "app", "app.development.yaml")
	if _, err := os.Stat(dropped); !os.IsNotExist(err) {
		t.Error("unselected development overlay was copied")
	}
}

// TestPackCommandRefusesNonEmptyOut verifies the command refuses a non-empty
// output directory by default.
func TestPackCommandRefusesNonEmptyOut(t *testing.T) {
	t.Parallel()

	cfg := writePackWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	seedPackDir(t, out)

	_, err := executePack(t, &mockPackFactory{}, cfg, "-e", "production", "--out", out)
	if !errors.Is(err, pack.ErrOutputNotEmpty) {
		t.Fatalf("err = %v, want ErrOutputNotEmpty", err)
	}
}

// TestPackCommandForceReplacesOut verifies --force lets the command replace a
// non-empty output directory.
func TestPackCommandForceReplacesOut(t *testing.T) {
	t.Parallel()

	cfg := writePackWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	stale := seedPackDir(t, out)

	_, err := executePack(
		t, &mockPackFactory{}, cfg, "-e", "production", "--out", out, "--force",
	)
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

	cfg := writePackWorkspace(t)

	_, err := executePack(t, &mockPackFactory{}, cfg, "-e", "production")
	if err == nil || !strings.Contains(err.Error(), "out") {
		t.Fatalf("err = %v, want a required --out error", err)
	}
}

// TestPackCommandFactoryError verifies a factory failure is returned unchanged.
func TestPackCommandFactoryError(t *testing.T) {
	t.Parallel()

	cfg := writePackWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	want := errors.New("factory failed")

	_, err := executePack(t, &mockPackFactory{err: want}, cfg, "--out", out)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestOutputPackReportsFilesAndRunHint verifies the summary names the
// destination, lists each file, and ends with the container run hint.
func TestOutputPackReportsFilesAndRunHint(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputPack(printer.NewPlain(&out, &errOut), pack.PackResult{
		OutDir:       "/dist",
		ManifestFile: "envx.yaml",
		Files:        []string{"app/app.yaml", "envx.yaml"},
		Environments: []string{"production"},
		Projects:     []string{"app"},
	})
	if err != nil {
		t.Fatalf("outputPack(): %v", err)
	}

	got := out.String()
	wants := []string{
		"Packed 2 files into /dist for environments [production]:",
		"  app/app.yaml",
		"envx run app --env production --config /dist/envx.yaml -- <command>",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("outputPack() = %q, want it to contain %q", got, want)
		}
	}
}

// TestFirstOr verifies the first element wins and the fallback covers an empty
// slice.
func TestFirstOr(t *testing.T) {
	t.Parallel()

	if got := firstOr([]string{"a", "b"}, "x"); got != "a" {
		t.Errorf("firstOr() = %q, want a", got)
	}
	if got := firstOr(nil, "x"); got != "x" {
		t.Errorf("firstOr(nil) = %q, want x", got)
	}
}
