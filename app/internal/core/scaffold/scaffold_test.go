package scaffold_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/core/workspace/filestore"
	"github.com/go-envx/envx/app/internal/lib/filex"
)

func TestCreateWritesTemplateFiles(t *testing.T) {
	t.Parallel()

	service := scaffold.Service{Templates: fstest.MapFS{
		"demo/envx.yaml":     {Data: []byte("root")},
		"demo/env/base.yaml": {Data: []byte("nested")},
		"other/ignored.yaml": {Data: []byte("other")},
	}}
	dir := t.TempDir()

	res, err := service.Create(scaffold.CreateParams{Template: "demo", TargetDir: dir})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	want := []string{
		filepath.Join(dir, "env", "base.yaml"),
		filepath.Join(dir, "envx.yaml"),
	}
	if len(res.Written) != len(want) {
		t.Fatalf("Written = %v, want %v", res.Written, want)
	}
	for i, path := range want {
		if res.Written[i] != path {
			t.Errorf("Written[%d] = %q, want %q", i, res.Written[i], path)
		}
	}
	data, err := filex.Read(filepath.Join(dir, "env", "base.yaml"))
	if err != nil {
		t.Fatalf("reading scaffolded file: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("content = %q, want %q", data, "nested")
	}
	if _, err := os.Stat(filepath.Join(dir, "ignored.yaml")); !os.IsNotExist(err) {
		t.Errorf("file from another template was written: %v", err)
	}
}

func TestCreateConflictPolicy(t *testing.T) {
	t.Parallel()

	service := scaffold.Service{Templates: fstest.MapFS{
		"demo/a.yaml": {Data: []byte("template")},
		"demo/b.yaml": {Data: []byte("template")},
	}}
	dir := t.TempDir()
	existing := filepath.Join(dir, "a.yaml")
	if err := os.WriteFile(existing, []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}

	params := scaffold.CreateParams{Template: "demo", TargetDir: dir}
	if _, err := service.Create(params); !errors.Is(err, scaffold.ErrConflict) {
		t.Fatalf("Create without force: err = %v, want ErrConflict", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.yaml")); !os.IsNotExist(err) {
		t.Errorf("conflict should write nothing, b.yaml stat err = %v", err)
	}

	params.Force = true
	if _, err := service.Create(params); err != nil {
		t.Fatalf("Create with force: %v", err)
	}
	data, err := filex.Read(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "template" {
		t.Errorf("forced content = %q, want %q", data, "template")
	}
}

func TestCreateUnknownTemplate(t *testing.T) {
	t.Parallel()

	service := scaffold.Service{Templates: fstest.MapFS{
		"demo/env/a.yaml": {Data: []byte("a")},
		"empty":           {Mode: os.ModeDir},
	}}
	for _, name := range []string{"", ".", "missing", "empty", "../demo", "demo/env"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			_, err := service.Create(scaffold.CreateParams{Template: name, TargetDir: dir})
			if !errors.Is(err, scaffold.ErrTemplateNotFound) {
				t.Fatalf("err = %v, want ErrTemplateNotFound", err)
			}
		})
	}
}

// TestQuickStartLoads guards that the embedded quick-start template produces a
// manifest the workspace loader accepts.
func TestQuickStartLoads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := scaffold.Service{}.Create(scaffold.CreateParams{
		Template:  scaffold.QuickStartTemplate,
		TargetDir: dir,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	repo, err := filestore.New(filestore.Params{Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	ws, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	project, ok := ws.LookupProject("api-service")
	if !ok {
		t.Fatal("api-service project missing")
	}
	for _, include := range project.IncludePaths {
		if _, err := os.Stat(include + ".yaml"); err != nil {
			t.Errorf("include %s not scaffolded: %v", include, err)
		}
	}
}
