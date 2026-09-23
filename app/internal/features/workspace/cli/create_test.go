package cli_test

import (
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/workspace/cli"
	"github.com/go-envx/envx/app/internal/features/workspace/command"
)

// mockCreateHandler implements cli.CreateHandler for unit testing.
type mockCreateHandler struct {
	result command.CreateWorkspaceResult
	err    error
	called bool
	cmd    command.CreateWorkspaceCommand
}

func (m *mockCreateHandler) Execute(
	cmd command.CreateWorkspaceCommand,
) (command.CreateWorkspaceResult, error) {
	m.called = true
	m.cmd = cmd
	return m.result, m.err
}

// TestNewCreateCmd builds the command tree with an injected handler and verifies
// execution maps flags into the command DTO.
func TestNewCreateCmd(t *testing.T) {
	t.Parallel()

	mock := &mockCreateHandler{
		result: command.CreateWorkspaceResult{
			Written: []string{"target/envx.yaml"},
		},
	}
	cmd := cli.NewCreateCmd(mock)
	cmd.SetArgs([]string{"quick-start", "--target-dir", "custom-dir", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	if !mock.called {
		t.Fatal("expected mock handler to be called")
	}
	if mock.cmd.Template != command.QuickStartTemplate {
		t.Errorf("Template = %q, want %q", mock.cmd.Template, command.QuickStartTemplate)
	}
	if mock.cmd.TargetDir != "custom-dir" {
		t.Errorf("TargetDir = %q, want custom-dir", mock.cmd.TargetDir)
	}
	if !mock.cmd.Force {
		t.Error("Force = false, want true")
	}
}

// TestSummary verifies the scaffold summary lists the written files and the
// first command to try, without a trailing newline so the printer owns it.
func TestSummary(t *testing.T) {
	t.Parallel()

	out := cli.Summary(
		command.QuickStartTemplate,
		"workspace",
		[]string{"workspace/envx.yaml"},
	)

	for _, want := range []string{
		"Scaffolded quick-start into workspace/ (1 files):",
		"  workspace/envx.yaml",
		"Try it:",
		"  cd workspace",
		"  envx get api-service DATABASE_HOST",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.HasSuffix(out, "\n") {
		t.Errorf("summary should not end with a newline:\n%q", out)
	}
}
