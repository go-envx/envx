package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/pack"
	"github.com/go-envx/envx/app/internal/core/secrets"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	workspaceStore "github.com/go-envx/envx/app/internal/core/workspace/filestore"
	"github.com/go-envx/envx/app/internal/lib/printer"
)

// fakeFactory builds a pack service from the workspace at the requested path
// over an empty secrets store.
type fakeFactory struct {
	// configPath records the manifest path the command requested.
	configPath string
	// err, when set, is returned instead of a service.
	err error
}

// emptySecrets is a pack.SecretStore holding no groups.
type emptySecrets struct{}

// NewPackService implements Factory.
func (f *fakeFactory) NewPackService(configPath string) (*pack.Service, error) {
	f.configPath = configPath
	if f.err != nil {
		return nil, f.err
	}
	repository, err := workspaceStore.New(workspaceStore.Params{Path: configPath})
	if err != nil {
		return nil, err
	}
	ws, err := repository.Load()
	if err != nil {
		return nil, err
	}
	return pack.New(*ws, pack.Dependencies{Secrets: emptySecrets{}})
}

// Read implements pack.SecretStore.
func (emptySecrets) Read(string) (secrets.Snapshot, error) {
	return secrets.Snapshot{}, nil
}

// Export implements pack.SecretStore.
func (emptySecrets) Export(io.Writer, []secrets.Group) error {
	return nil
}

// executePack builds the pack command, executes it with args, and returns its
// captured stdout.
func executePack(
	t *testing.T, factory Factory, configPath string, args ...string,
) (stdout string, err error) {
	t.Helper()

	cmd := NewPackCommand(factory)
	workspacecli.BindConfig(cmd)
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", configPath}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), err
}

// writePackWorkspace scaffolds a two-project workspace in a fresh temp dir and
// returns the manifest path.
func writePackWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"envx.yaml": "environments: [development, production]\n" +
			"projects:\n" +
			"  app:\n    includes: [env/app]\n" +
			"  web:\n    includes: [env/web]\n",
		"env/app.yaml":             "A: base\n",
		"env/app.development.yaml": "A: dev\n",
		"env/app.production.yaml":  "A: prod\n",
		"env/web.yaml":             "W: base\n",
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

// TestPackCommandWritesBundle verifies the command passes --config to the
// factory, copies only the selected environment's files, and reports them.
func TestPackCommandWritesBundle(t *testing.T) {
	t.Parallel()

	cfg := writePackWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	factory := &fakeFactory{}

	stdout, err := executePack(t, factory, cfg, "-e", "production", "--out", out)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if factory.configPath != cfg {
		t.Errorf("factory configPath = %q, want %q", factory.configPath, cfg)
	}
	if !strings.Contains(stdout, "app.production.yaml") {
		t.Errorf("summary missing packed file, got %q", stdout)
	}
	kept := filepath.Join(out, "app", "app.production.yaml")
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("selected production overlay was not copied: %v", err)
	}
	dropped := filepath.Join(out, "app", "app.development.yaml")
	if _, err := os.Stat(dropped); !os.IsNotExist(err) {
		t.Error("unselected development overlay was copied")
	}
}

// TestPackCommandProjectFlag verifies -p narrows the bundle to the named
// project.
func TestPackCommandProjectFlag(t *testing.T) {
	t.Parallel()

	cfg := writePackWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	stdout, err := executePack(t, &fakeFactory{}, cfg, "-p", "web", "--out", out)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if !strings.Contains(stdout, "web.yaml") || strings.Contains(stdout, "app.yaml") {
		t.Errorf("summary = %q, want only the web project's files", stdout)
	}
}

// TestPackCommandRepeatedEnv verifies -e is repeatable.
func TestPackCommandRepeatedEnv(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "dist")
	stdout, err := executePack(
		t, &fakeFactory{}, writePackWorkspace(t),
		"-e", "development", "-e", "production", "--out", out,
	)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if !strings.Contains(stdout, "for environments [development, production]") {
		t.Errorf("summary = %q, want both environments", stdout)
	}
}

// TestPackCommandForce verifies a non-empty --out is refused unless --force is
// set, which replaces it.
func TestPackCommandForce(t *testing.T) {
	t.Parallel()

	cfg := writePackWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	stale := seedPackDir(t, out)

	_, err := executePack(t, &fakeFactory{}, cfg, "--out", out)
	if !errors.Is(err, pack.ErrOutputNotEmpty) {
		t.Fatalf("without --force: err = %v, want ErrOutputNotEmpty", err)
	}

	if _, err := executePack(t, &fakeFactory{}, cfg, "--out", out, "--force"); err != nil {
		t.Fatalf("with --force: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("--force did not clear the stale file")
	}
}

// TestPackCommandRequiresOut verifies --out is a required flag checked before
// the factory is called.
func TestPackCommandRequiresOut(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{}
	_, err := executePack(t, factory, writePackWorkspace(t), "-e", "production")
	if err == nil || !strings.Contains(err.Error(), `required flag(s) "out"`) {
		t.Fatalf("err = %v, want a required --out error", err)
	}
	if factory.configPath != "" {
		t.Error("factory was called despite the missing --out")
	}
}

// TestPackCommandFactoryError verifies a factory failure is returned unchanged.
func TestPackCommandFactoryError(t *testing.T) {
	t.Parallel()

	want := errors.New("factory failed")
	out := filepath.Join(t.TempDir(), "dist")

	_, err := executePack(t, &fakeFactory{err: want}, "envx.yaml", "--out", out)
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
