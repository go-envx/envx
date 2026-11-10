package pack

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"gopkg.in/yaml.v3"
)

// fixture bundles a workspace on disk with the pack fakes.
type fixture struct {
	// root is the workspace directory.
	root string
	// ws is the workspace configuration passed to New.
	ws workspace.Workspace
	// store is the in-memory secrets store.
	store *memSecrets
}

// newFixture returns an empty workspace in a temporary directory.
func newFixture(t *testing.T, environments ...string) *fixture {
	t.Helper()
	root := t.TempDir()
	return &fixture{
		root: root,
		ws: workspace.Workspace{
			Path:         filepath.Join(root, "envx.yaml"),
			Root:         root,
			Environments: environments,
			Projects:     make(map[string]workspace.Project),
		},
		store: newMemSecrets(),
	}
}

// newSampleFixture returns a two-project workspace with development and
// production overlays and a store holding referenced and unreferenced values.
func newSampleFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t, "development", "production")
	f.write(t, "envx.yaml", "environments: [development, production]\n"+
		"projects:\n"+
		"  api:\n    includes: [env/app, env/api]\n"+
		"  web:\n    includes: [env/app]\n")
	f.project("api", "env/app", "env/api")
	f.project("web", "env/app")
	f.write(t, "env/app.yaml", "A: base\nTOKEN: secret://shared/token\n")
	f.write(t, "env/app.development.yaml", "A: dev\n")
	f.write(t, "env/app.production.yaml", "A: prod\n")
	f.write(t, "env/api.yaml", "B: base\n")
	f.write(t, "env/api.development.yaml", "B: dev\n")
	f.useStore(t, "secrets.yaml",
		secrets.Group{
			Name:      "shared",
			PublicKey: "PUBKEY",
			Values:    map[string]string{"token": "t0k", "unused": "nope"},
		},
		secrets.Group{Name: "other", Values: map[string]string{"x": "y"}},
	)
	return f
}

// write writes body at rel under the workspace root and returns its path.
func (f *fixture) write(t *testing.T, rel, body string) string {
	t.Helper()
	return writeFile(t, f.root, rel, body)
}

// project declares a project with includes relative to the root.
func (f *fixture) project(name string, includes ...string) {
	paths := make([]string, len(includes))
	for i, rel := range includes {
		paths[i] = filepath.Join(f.root, rel)
	}
	f.ws.Projects[name] = workspace.Project{Includes: includes, IncludePaths: paths}
}

// useStore creates the store file at rel and seeds the reader with groups.
func (f *fixture) useStore(t *testing.T, rel string, groups ...secrets.Group) {
	t.Helper()
	f.ws.Secrets.SecretsPath = f.write(t, rel, "secrets: {}\n")
	f.store = newMemSecrets(groups...)
}

// service constructs a Service over the fixture.
func (f *fixture) service(t *testing.T) *Service {
	t.Helper()
	service, err := New(f.ws, Dependencies{Secrets: f.store})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return service
}

// pack runs Pack and fails the test on error.
func (f *fixture) pack(t *testing.T, params Params) Result {
	t.Helper()
	result, err := f.service(t).Pack(params)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	return result
}

// writeFile writes body at rel under dir, creating parents, and returns its path.
func writeFile(t *testing.T, dir, rel, body string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// readFile reads rel under dir and fails the test on error.
func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := filex.Read(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// listFiles returns the sorted slash-separated files under dir.
func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		files = append(files, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	return files
}

// manifestProjects parses the bundle manifest's projects and their includes.
func manifestProjects(t *testing.T, outDir string) map[string][]string {
	t.Helper()
	var doc struct {
		Projects map[string]struct {
			Includes []string `yaml:"includes"`
		} `yaml:"projects"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, outDir, "envx.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	projects := make(map[string][]string, len(doc.Projects))
	for name, p := range doc.Projects {
		projects[name] = p.Includes
	}
	return projects
}

// present fails the test unless rel exists under dir.
func present(t *testing.T, dir, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Errorf("expected %s in bundle: %v", rel, err)
	}
}

// absent fails the test unless rel is missing under dir.
func absent(t *testing.T, dir, rel string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	if !os.IsNotExist(err) {
		t.Errorf("expected %s absent from bundle, stat err = %v", rel, err)
	}
}
