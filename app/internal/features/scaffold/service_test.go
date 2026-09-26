package scaffold_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/scaffold"
)

// TestNewServiceRequiresSource verifies NewService rejects a nil Source fs.
func TestNewServiceRequiresSource(t *testing.T) {
	t.Parallel()

	_, err := scaffold.NewService(scaffold.ServiceParams{})
	if err == nil {
		t.Fatal("expected error when Source is nil")
	}
}

// TestCreateScaffoldsFiles verifies each template writes its envx.yaml and nested
// namespace files into the target directory.
func TestCreateScaffoldsFiles(t *testing.T) {
	t.Parallel()

	service, err := scaffold.NewService(scaffold.ServiceParams{
		Source: scaffold.TemplatesFS,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	for _, name := range []string{scaffold.QuickStartTemplate} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			res, err := service.Create(scaffold.CreateParams{
				Template:  name,
				TargetDir: dir,
			})
			if err != nil {
				t.Fatalf("service.Create: %v", err)
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

// TestCreateRefusesOverwrite verifies a second scaffold over existing files fails
// without Force and succeeds with it.
func TestCreateRefusesOverwrite(t *testing.T) {
	t.Parallel()

	service, err := scaffold.NewService(scaffold.ServiceParams{
		Source: scaffold.TemplatesFS,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	dir := t.TempDir()
	params := scaffold.CreateParams{
		Template:  scaffold.QuickStartTemplate,
		TargetDir: dir,
	}
	if _, err := service.Create(params); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	_, err = service.Create(params)
	if err == nil {
		t.Fatal("expected a conflict error on the second scaffold without --force")
	}
	if !errors.Is(err, scaffold.ErrConflict) {
		t.Errorf("expected ErrConflict, got: %v", err)
	}

	forced := scaffold.CreateParams{
		Template:  scaffold.QuickStartTemplate,
		TargetDir: dir,
		Force:     true,
	}
	if _, err := service.Create(forced); err != nil {
		t.Fatalf("force scaffold: %v", err)
	}
}

// TestCreateUnknownTemplate verifies requesting a non-existent template fails.
func TestCreateUnknownTemplate(t *testing.T) {
	t.Parallel()

	service, err := scaffold.NewService(scaffold.ServiceParams{
		Source: scaffold.TemplatesFS,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	dir := t.TempDir()
	_, err = service.Create(scaffold.CreateParams{
		Template:  "non-existent",
		TargetDir: dir,
	})
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
	if !errors.Is(err, scaffold.ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got: %v", err)
	}
}

// TestQuickStartResolves scaffolds the quick-start workspace and resolves it,
// guarding that the scaffolded files load and that api-service resolves the
// database host the getting-started guide and the "try it" hint rely on.
func TestQuickStartResolves(t *testing.T) {
	t.Parallel()

	service, err := scaffold.NewService(scaffold.ServiceParams{
		Source: scaffold.TemplatesFS,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	dir := t.TempDir()
	params := scaffold.CreateParams{
		Template:  scaffold.QuickStartTemplate,
		TargetDir: dir,
	}
	if _, err := service.Create(params); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	manifestPath := filepath.Join(dir, "envx.yaml")
	in := &core.Input{ConfigPath: &manifestPath}
	resolved, err := core.ResolveProject(in, "api-service")
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
