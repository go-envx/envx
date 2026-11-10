package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/printer"
)

// fakeFactory builds a scaffold service over an in-memory template.
type fakeFactory struct {
	// err, when set, is returned instead of a service.
	err error
}

// NewScaffoldService implements Factory.
func (f *fakeFactory) NewScaffoldService() (*scaffold.Service, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &scaffold.Service{
		Templates: fstest.MapFS{
			scaffold.QuickStartTemplate + "/envx.yaml": {Data: []byte("template\n")},
		},
	}, nil
}

// executeCreate runs "create" with args and returns its captured stdout.
func executeCreate(t *testing.T, factory Factory, args ...string) (string, error) {
	t.Helper()

	cmd := NewCreateCommand(factory)
	cmd.SilenceUsage = true
	cmd.SetArgs(args)

	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.Execute()
	return out.String(), err
}

// TestCreateCommandWritesTargetDir verifies --target-dir selects the
// destination and the summary is printed.
func TestCreateCommandWritesTargetDir(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "custom-dir")
	stdout, err := executeCreate(
		t, &fakeFactory{}, scaffold.QuickStartTemplate, "--target-dir", target,
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "envx.yaml")); err != nil {
		t.Errorf("envx.yaml not written: %v", err)
	}
	if !strings.Contains(stdout, "Scaffolded quick-start into "+target) {
		t.Errorf("stdout = %q, want the scaffold summary", stdout)
	}
}

// TestCreateCommandDefaultTargetDir verifies the target directory defaults to
// the template name in the working directory.
func TestCreateCommandDefaultTargetDir(t *testing.T) {
	// Not parallel: t.Chdir changes the process working directory.
	dir := t.TempDir()
	t.Chdir(dir)

	_, err := executeCreate(t, &fakeFactory{}, scaffold.QuickStartTemplate)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	want := filepath.Join(dir, scaffold.QuickStartTemplate, "envx.yaml")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("%s not written: %v", want, err)
	}
}

// TestCreateCommandForce verifies a conflicting target is refused unless
// --force is set.
func TestCreateCommandForce(t *testing.T) {
	t.Parallel()

	target := t.TempDir()
	existing := filepath.Join(target, "envx.yaml")
	if err := os.WriteFile(existing, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := executeCreate(
		t, &fakeFactory{}, scaffold.QuickStartTemplate, "--target-dir", target,
	)
	if !errors.Is(err, scaffold.ErrConflict) {
		t.Fatalf("without --force: err = %v, want ErrConflict", err)
	}

	_, err = executeCreate(
		t, &fakeFactory{},
		scaffold.QuickStartTemplate, "--target-dir", target, "--force",
	)
	if err != nil {
		t.Fatalf("with --force: %v", err)
	}
	data, err := filex.Read(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "template\n" {
		t.Errorf("envx.yaml = %q, want the template content", data)
	}
}

// TestCreateCommandFactoryError verifies a factory failure is returned
// unchanged.
func TestCreateCommandFactoryError(t *testing.T) {
	t.Parallel()

	want := errors.New("factory failure")
	_, err := executeCreate(
		t, &fakeFactory{err: want},
		scaffold.QuickStartTemplate, "--target-dir", t.TempDir(),
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestOutputCreate verifies the scaffold summary lists the written files and the
// first command to try.
func TestOutputCreate(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	console := printer.New(printer.Options{
		Out: &buf,
	})

	err := outputCreate(
		console,
		scaffold.QuickStartTemplate,
		"workspace",
		scaffold.CreateResult{
			Written: []string{"workspace/envx.yaml"},
		},
	)
	if err != nil {
		t.Fatalf("outputCreate(): %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Scaffolded quick-start into workspace/ (1 files):",
		"  workspace/envx.yaml",
		"Try it:",
		"  cd workspace",
		"  envx get api-service DATABASE_HOST",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
