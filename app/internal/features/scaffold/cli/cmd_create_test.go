package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// mockScaffoldFactory implements Factory for unit testing.
type mockScaffoldFactory struct {
	svc *scaffold.Service
	err error
}

func (m *mockScaffoldFactory) ScaffoldService() (*scaffold.Service, error) {
	return m.svc, m.err
}

// newTestScaffoldService constructs a real scaffold service backed by embedded
// templates.
func newTestScaffoldService(t *testing.T) *scaffold.Service {
	t.Helper()
	svc, err := scaffold.NewService(scaffold.ServiceParams{
		Source: scaffold.TemplatesFS,
	})
	if err != nil {
		t.Fatalf("scaffold.NewService(): %v", err)
	}
	return svc
}

// TestNewCreateCommand verifies the command builds and dispatches through the factory.
func TestNewCreateCommand(t *testing.T) {
	t.Parallel()

	t.Run("with custom flags", func(t *testing.T) {
		t.Parallel()

		svc := newTestScaffoldService(t)
		factory := &mockScaffoldFactory{svc: svc}
		parent := NewCreateCommand(factory)

		targetDir := filepath.Join(t.TempDir(), "custom-dir")
		parent.SetArgs([]string{"quick-start", "--target-dir", targetDir, "--force"})

		var out bytes.Buffer
		parent.SetOut(&out)

		if err := parent.Execute(); err != nil {
			t.Fatalf("parent.Execute(): %v", err)
		}

		if _, err := os.Stat(filepath.Join(targetDir, "envx.yaml")); err != nil {
			t.Errorf("expected envx.yaml to exist: %v", err)
		}

		output := out.String()
		if !strings.Contains(output, "Scaffolded quick-start into") {
			t.Errorf("output missing summary: %q", output)
		}
	})

	t.Run("with factory error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("factory failure")
		factory := &mockScaffoldFactory{err: expectedErr}
		cmd := NewCreateCommand(factory)
		cmd.SetArgs([]string{
			"quick-start",
			"--target-dir",
			filepath.Join(t.TempDir(), "test"),
		})

		err := cmd.Execute()
		if !errors.Is(err, expectedErr) {
			t.Fatalf("got err = %v, want %v", err, expectedErr)
		}
	})
}

// TestNewCreateCommandDefaultFlags verifies the default target directory.
func TestNewCreateCommandDefaultFlags(t *testing.T) {
	// Not parallel because t.Chdir modifies process working directory.
	tmp := t.TempDir()
	t.Chdir(tmp)

	svc := newTestScaffoldService(t)
	factory := &mockScaffoldFactory{svc: svc}
	cmd := NewCreateCommand(factory)
	cmd.SetArgs([]string{"quick-start"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute(): %v", err)
	}

	expectedFile := filepath.Join(tmp, scaffold.QuickStartTemplate, "envx.yaml")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected %s to exist: %v", expectedFile, err)
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
