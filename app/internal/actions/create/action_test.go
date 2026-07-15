package create

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
)

// -------------------------------------------------------------------------------------

// TestExecuteScaffoldsFiles verifies each template writes its envx.yaml and nested
// namespace files into the target directory.
func TestExecuteScaffoldsFiles(t *testing.T) {
	t.Parallel()

	for _, name := range []string{quickStart, exampleWorkspace} {
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

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

// TestExampleWorkspaceResolves scaffolds the example workspace and resolves it,
// guarding the outputs the docs rely on: the development overlay wins the database
// host, the last-included project namespace wins the PORT collision, and a list
// leaf joins with the default delimiter.
func TestExampleWorkspaceResolves(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := actionParams{Template: exampleWorkspace, TargetDir: dir}
	if _, err := execute(p); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	manifestPath := filepath.Join(dir, "envx.yaml")
	in := &config.Input{ConfigPath: &manifestPath}
	resolved, err := config.Resolve(in, "api-service")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	env, err := envmerge.Build(resolved.Envmerge)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	cases := map[string]string{
		"DATABASE_HOST":       "dev-db.local",
		"PORT":                "3001",
		"DATABASE_POOL_HOSTS": "db-a.local,db-b.local",
	}
	for key, want := range cases {
		if got, _ := env.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
