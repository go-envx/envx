package create

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
)

// TestSummary verifies the scaffold summary lists the written files and the
// first command to try, without a trailing newline so the printer owns it.
func TestSummary(t *testing.T) {
	t.Parallel()

	out := summary(quickStart, "workspace", []string{"workspace/envx.yaml"})

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

// TestExecuteScaffoldsFiles verifies each template writes its envx.yaml and nested
// namespace files into the target directory.
func TestExecuteScaffoldsFiles(t *testing.T) {
	t.Parallel()

	for _, name := range []string{quickStart} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			res, err := execute(actionParams{Template: name, TargetDir: dir})
			if err != nil {
				t.Fatalf("execute: %v", err)
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

	dir := t.TempDir()
	if _, err := execute(actionParams{Template: quickStart, TargetDir: dir}); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	if _, err := execute(actionParams{Template: quickStart, TargetDir: dir}); err == nil {
		t.Fatal("expected a conflict error on the second scaffold without --force")
	}
	forced := actionParams{Template: quickStart, TargetDir: dir, Force: true}
	if _, err := execute(forced); err != nil {
		t.Fatalf("force scaffold: %v", err)
	}
}

// TestQuickStartResolves scaffolds the quick-start workspace and resolves it,
// guarding that the scaffolded files load and that api-service resolves the
// database host the getting-started guide and the "try it" hint rely on.
func TestQuickStartResolves(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := actionParams{Template: quickStart, TargetDir: dir}
	if _, err := execute(p); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	manifestPath := filepath.Join(dir, "envx.yaml")
	in := &config.Input{ConfigPath: &manifestPath}
	resolved, err := config.ResolveProject(in, "api-service")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	entry, err := resolved.Envmerge.Get(envmerge.GetParams{Key: "DATABASE_HOST"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if entry.Value != "localhost" {
		t.Errorf("DATABASE_HOST = %q, want %q", entry.Value, "localhost")
	}
}
