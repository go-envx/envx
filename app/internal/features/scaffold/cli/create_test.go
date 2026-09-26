package cli_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/features/scaffold/cli"
)

// mockScaffoldService implements the scaffold service interface for unit testing.
type mockScaffoldService struct {
	result scaffold.CreateResult
	err    error
	called bool
	params scaffold.CreateParams
}

func (m *mockScaffoldService) Create(
	params scaffold.CreateParams,
) (scaffold.CreateResult, error) {
	m.called = true
	m.params = params
	return m.result, m.err
}

// TestNewCreateCmd builds the command tree with an injected service and verifies
// execution maps flags into the command DTO.
func TestNewCreateCmd(t *testing.T) {
	t.Parallel()

	t.Run("with custom flags", func(t *testing.T) {
		mock := &mockScaffoldService{
			result: scaffold.CreateResult{
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
		if mock.params.Template != scaffold.QuickStartTemplate {
			t.Errorf("Template = %q, want %q", mock.params.Template, scaffold.QuickStartTemplate)
		}
		if mock.params.TargetDir != "custom-dir" {
			t.Errorf("TargetDir = %q, want custom-dir", mock.params.TargetDir)
		}
		if !mock.params.Force {
			t.Error("Force = false, want true")
		}
	})

	t.Run("with default flags", func(t *testing.T) {
		mock := &mockScaffoldService{
			result: scaffold.CreateResult{
				Written: []string{"quick-start/envx.yaml"},
			},
		}
		cmd := cli.NewCreateCmd(mock)
		cmd.SetArgs([]string{"quick-start"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute: %v", err)
		}

		if !mock.called {
			t.Fatal("expected mock handler to be called")
		}
		if mock.params.Template != scaffold.QuickStartTemplate {
			t.Errorf(
				"Template = %q, want %q",
				mock.params.Template,
				scaffold.QuickStartTemplate,
			)
		}
		if mock.params.TargetDir != scaffold.QuickStartTemplate {
			t.Errorf(
				"TargetDir = %q, want %q",
				mock.params.TargetDir,
				scaffold.QuickStartTemplate,
			)
		}
		if mock.params.Force {
			t.Error("Force = true, want false")
		}
	})
}
