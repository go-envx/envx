package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/workspace"
)

// TestExecuteScaffoldsFiles verifies each template writes its envx.yaml and nested
// namespace files into the target directory.
func TestExecuteScaffoldsFiles(t *testing.T) {
	t.Parallel()

	handler := workspace.NewCreateWorkspaceHandler()
	for _, name := range []string{workspace.QuickStartTemplate} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			res, err := handler.Execute(workspace.CreateWorkspaceCommand{
				Template:  name,
				TargetDir: dir,
			})
			if err != nil {
				t.Fatalf("CreateWorkspaceHandler.Execute: %v", err)
			}
			if len(res.Written) == 0 {
				t.Fatal("expected files to be written")
			}
			if _, err := os.Stat(filepath.Join(dir, "envx.yaml")); err != nil {
				t.Errorf("envx.yaml not scaffolded: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "env", "database.yaml")); err != nil {
				t.Errorf("env/database.yaml not scaffolded: %v", err)
			}
		})
	}
}

// TestExecuteRefusesOverwrite verifies a second scaffold over existing files fails
// without --force and succeeds with it.
func TestExecuteRefusesOverwrite(t *testing.T) {
	t.Parallel()

	handler := workspace.NewCreateWorkspaceHandler()
	dir := t.TempDir()
	cmd := workspace.CreateWorkspaceCommand{
		Template:  workspace.QuickStartTemplate,
		TargetDir: dir,
	}
	if _, err := handler.Execute(cmd); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	if _, err := handler.Execute(cmd); err == nil {
		t.Fatal("expected a conflict error on the second scaffold without --force")
	}
	forced := workspace.CreateWorkspaceCommand{
		Template:  workspace.QuickStartTemplate,
		TargetDir: dir,
		Force:     true,
	}
	if _, err := handler.Execute(forced); err != nil {
		t.Fatalf("force scaffold: %v", err)
	}
}

// TestQuickStartResolves scaffolds the quick-start workspace and resolves it,
// guarding that the scaffolded files load and that api-service resolves the
// database host the getting-started guide and the "try it" hint rely on.
func TestQuickStartResolves(t *testing.T) {
	t.Parallel()

	handler := workspace.NewCreateWorkspaceHandler()
	dir := t.TempDir()
	cmd := workspace.CreateWorkspaceCommand{
		Template:  workspace.QuickStartTemplate,
		TargetDir: dir,
	}
	if _, err := handler.Execute(cmd); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	manifestPath := filepath.Join(dir, "envx.yaml")
	in := &config.Input{ConfigPath: &manifestPath}
	resolved, err := config.ResolveProject(in, "api-service")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	entry, err := resolved.Envmerge.Get(env.GetParams{Key: "DATABASE_HOST"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if entry.Value != "localhost" {
		t.Errorf("DATABASE_HOST = %q, want %q", entry.Value, "localhost")
	}
}
