package pack

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeSource writes body to path under dir, creating parent directories, and
// returns the absolute path.
func writeSource(t *testing.T, dir, path, body string) string {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return full
}

// newWorkspace scaffolds a two-project workspace whose namespaces live in nested
// directories, with base files and overlays for development and production, plus a
// secrets store, and returns the root dir and a Workspace describing it.
func newWorkspace(t *testing.T) (string, Workspace) {
	t.Helper()
	root := t.TempDir()

	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development, production]\n"+
			"projects:\n"+
			"  api:\n    includes: [env/app, env/api]\n"+
			"  web:\n    includes: [env/app]\n")
	// env/app references one secret; the store also holds an unused entry, a whole
	// unused group, and public keys, all of which filtering must drop.
	writeSource(t, root, "env/app.yaml", "A: base\nTOKEN: secret://shared/token\n")
	writeSource(t, root, "env/app.development.yaml", "A: dev\n")
	writeSource(t, root, "env/app.production.yaml", "A: prod\n")
	writeSource(t, root, "env/api.yaml", "B: base\n")
	writeSource(t, root, "env/api.development.yaml", "B: dev\n")
	store := "public_keys:\n  shared: PUBKEY\n" +
		"secrets:\n" +
		"  shared:\n    token: t0k\n    unused: nope\n" +
		"  other:\n    x: y\n"
	secrets := writeSource(t, root, "secrets.yaml", store)

	return root, Workspace{
		ManifestPath: manifest,
		Root:         root,
		SecretsPath:  secrets,
		Environments: []string{"development", "production"},
		Projects: []Project{
			{Name: "api", Includes: []string{"env/app", "env/api"}},
			{Name: "web", Includes: []string{"env/app"}},
		},
	}
}

// bundleFiles returns the sorted set of bundle-relative file paths under outDir.
func bundleFiles(t *testing.T, outDir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, relErr := filepath.Rel(outDir, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// projectIncludes parses the bundle manifest and returns the named project's
// includes, so a test can assert they were rewritten to per-project paths.
func projectIncludes(t *testing.T, manifestPath, project string) []string {
	t.Helper()
	data, err := os.ReadFile(manifestPath) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Projects map[string]struct {
			Includes []string `yaml:"includes"`
		} `yaml:"projects"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Projects[project].Includes
}

// manifestProjects parses the bundle manifest and returns its declared project
// names, sorted, so a test can assert unselected projects were pruned.
func manifestProjects(t *testing.T, manifestPath string) []string {
	t.Helper()
	data, err := os.ReadFile(manifestPath) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Projects map[string]yaml.Node `yaml:"projects"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(doc.Projects))
	for name := range doc.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// readFile reads a bundle file and fails the test on error.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// mustPack runs Pack and fails the test on error, returning the result.
func mustPack(t *testing.T, ws Workspace, p Params) Result {
	t.Helper()
	result, err := Pack(ws, p)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	return result
}

// present fails the test unless rel exists under dir.
func present(t *testing.T, dir, rel string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s in bundle: %v", rel, err)
	}
}

// absent fails the test unless rel is missing under dir.
func absent(t *testing.T, dir, rel string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s absent from bundle, stat err = %v", rel, err)
	}
}

// TestPackWritesPerProjectDirectories verifies a single-environment pack writes
// each project's namespace files under its own <project>/ directory, with the
// manifest and secrets store at the bundle root, excluding the unselected overlay
// and the keys. A namespace both projects include (env/app) is copied into both
// directories.
func TestPackWritesPerProjectDirectories(t *testing.T) {
	t.Parallel()
	root, ws := newWorkspace(t)
	writeSource(t, root, "envx.keys", "shared: PRIVATE\n") // must never be copied
	out := filepath.Join(t.TempDir(), "dist")

	result := mustPack(t, ws, Params{Environments: []string{"production"}, OutDir: out})

	want := []string{
		"api/api.yaml",
		"api/app.production.yaml",
		"api/app.yaml",
		"envx.yaml",
		"secrets.yaml",
		"web/app.production.yaml",
		"web/app.yaml",
	}
	if got := bundleFiles(t, out); !equal(got, want) {
		t.Errorf("bundle files = %v, want %v", got, want)
	}
	if !equal(result.Files, want) {
		t.Errorf("result files = %v, want %v", result.Files, want)
	}
	absent(t, out, "envx.keys")
	absent(t, out, "api/app.development.yaml")
}

// TestPackRewritesManifestIncludes verifies the bundle manifest's includes are
// rewritten to their per-project paths, so run resolves them against each
// project's bundle directory.
func TestPackRewritesManifestIncludes(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	bundleManifest := filepath.Join(out, "envx.yaml")
	api := projectIncludes(t, bundleManifest, "api")
	if !equal(api, []string{"api/app", "api/api"}) {
		t.Errorf("api includes = %v, want [api/app api/api]", api)
	}
	web := projectIncludes(t, bundleManifest, "web")
	if !equal(web, []string{"web/app"}) {
		t.Errorf("web includes = %v, want [web/app]", web)
	}
}

// TestPackSeparatesCollidingBasenamesByProject verifies two projects that each
// include a differently-located namespace sharing a basename keep their own copy
// under their own directory, with no cross-project disambiguation.
func TestPackSeparatesCollidingBasenamesByProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development]\n"+
			"projects:\n"+
			"  one:\n    includes: [env/app]\n"+
			"  two:\n    includes: [svc/app]\n")
	writeSource(t, root, "env/app.yaml", "A: 1\n")
	writeSource(t, root, "svc/app.yaml", "A: 2\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		Environments: []string{"development"},
		Projects: []Project{
			{Name: "one", Includes: []string{"env/app"}},
			{Name: "two", Includes: []string{"svc/app"}},
		},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	// Each project keeps the plain "app.yaml" name inside its own directory.
	present(t, out, "one/app.yaml")
	present(t, out, "two/app.yaml")
	bundleManifest := filepath.Join(out, "envx.yaml")
	first := projectIncludes(t, bundleManifest, "one")
	if !equal(first, []string{"one/app"}) {
		t.Errorf("project one includes = %v, want [one/app]", first)
	}
	second := projectIncludes(t, bundleManifest, "two")
	if !equal(second, []string{"two/app"}) {
		t.Errorf("project two includes = %v, want [two/app]", second)
	}
	// The two source files, though identically named, land as distinct files.
	one := readFile(t, filepath.Join(out, "one", "app.yaml"))
	two := readFile(t, filepath.Join(out, "two", "app.yaml"))
	if bytes.Equal(one, two) {
		t.Error("colliding namespaces were merged into one file")
	}
}

// TestPackDisambiguatesIntraProjectCollision verifies that when one project
// includes two differently-located namespaces sharing a basename, the second is
// disambiguated within the project directory and the manifest points at it.
func TestPackDisambiguatesIntraProjectCollision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development]\n"+
			"projects:\n"+
			"  one:\n    includes: [env/app, svc/app]\n")
	writeSource(t, root, "env/app.yaml", "A: 1\n")
	writeSource(t, root, "svc/app.yaml", "A: 2\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		Environments: []string{"development"},
		Projects: []Project{
			{Name: "one", Includes: []string{"env/app", "svc/app"}},
		},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	// "env/app" comes first and keeps "app"; "svc/app" becomes "app-2".
	present(t, out, "one/app.yaml")
	present(t, out, "one/app-2.yaml")
	got := projectIncludes(t, filepath.Join(out, "envx.yaml"), "one")
	if !equal(got, []string{"one/app", "one/app-2"}) {
		t.Errorf("project one includes = %v, want [one/app one/app-2]", got)
	}
	first := readFile(t, filepath.Join(out, "one", "app.yaml"))
	second := readFile(t, filepath.Join(out, "one", "app-2.yaml"))
	if bytes.Equal(first, second) {
		t.Error("colliding namespaces were merged into one file")
	}
}

// TestPackDisambiguatesBaseOverlayCollision verifies a namespace's overlay
// cannot overwrite another namespace's base file when their complete output
// filenames would otherwise match.
func TestPackDisambiguatesBaseOverlayCollision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [production]\n"+
			"projects:\n  one:\n    includes: [env/app, svc/app.production]\n")
	writeSource(t, root, "env/app.yaml", "A: base\n")
	writeSource(t, root, "env/app.production.yaml", "A: overlay\n")
	writeSource(t, root, "svc/app.production.yaml", "B: other\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		Environments: []string{"production"},
		Projects: []Project{{
			Name:     "one",
			Includes: []string{"env/app", "svc/app.production"},
		}},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	present(t, out, "one/app.production.yaml")
	present(t, out, "one/app.production-2.yaml")
	got := projectIncludes(t, filepath.Join(out, "envx.yaml"), "one")
	if !equal(got, []string{"one/app", "one/app.production-2"}) {
		t.Errorf("project one includes = %v, want [one/app one/app.production-2]", got)
	}
	if got := string(readFile(
		t, filepath.Join(out, "one", "app.production.yaml"),
	)); got != "A: overlay\n" {
		t.Errorf("overlay content = %q, want %q", got, "A: overlay\n")
	}
	if got := string(readFile(
		t, filepath.Join(out, "one", "app.production-2.yaml"),
	)); got != "B: other\n" {
		t.Errorf("base content = %q, want %q", got, "B: other\n")
	}
}

// TestPackSanitizesProjectDirectoryName verifies a project name that is not
// filesystem-safe is sanitized into a single safe directory component and the
// manifest includes are rewritten to match.
func TestPackSanitizesProjectDirectoryName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development]\n"+
			"projects:\n"+
			"  \"team/api\":\n    includes: [env/app]\n")
	writeSource(t, root, "env/app.yaml", "A: 1\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		Environments: []string{"development"},
		Projects: []Project{
			{Name: "team/api", Includes: []string{"env/app"}},
		},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	present(t, out, "team_api/app.yaml")
	got := projectIncludes(t, filepath.Join(out, "envx.yaml"), "team/api")
	if !equal(got, []string{"team_api/app"}) {
		t.Errorf("includes = %v, want [team_api/app]", got)
	}
}

// TestPackHandlesEscapingInclude verifies an include that resolves outside the
// workspace root is copied into the project's bundle directory under its base
// name, dropping the original directory structure.
func TestPackHandlesEscapingInclude(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development]\n"+
			"projects:\n  escape:\n    includes: ['../outside']\n")
	writeSource(t, parent, "outside.yaml", "X: 1\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		Environments: []string{"development"},
		Projects:     []Project{{Name: "escape", Includes: []string{"../outside"}}},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{Environments: []string{"development"}, OutDir: out})
	present(t, out, "escape/outside.yaml")
	got := projectIncludes(t, filepath.Join(out, "envx.yaml"), "escape")
	if !equal(got, []string{"escape/outside"}) {
		t.Errorf("escape includes = %v, want [escape/outside]", got)
	}
}

// TestPackMultipleEnvironmentsKeepsEveryOverlay verifies selecting several
// environments keeps each selected environment's overlays.
func TestPackMultipleEnvironmentsKeepsEveryOverlay(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{
		Environments: []string{"production", "development"},
		OutDir:       out,
	})

	present(t, out, "api/app.development.yaml")
	present(t, out, "api/app.production.yaml")
	present(t, out, "api/api.development.yaml")
	present(t, out, "web/app.development.yaml")
}

// TestPackDefaultsToAllDeclaredEnvironments verifies an empty environment
// selection copies every declared environment's overlays.
func TestPackDefaultsToAllDeclaredEnvironments(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	result := mustPack(t, ws, Params{OutDir: out})
	if !equal(result.Environments, []string{"development", "production"}) {
		t.Errorf("environments = %v, want all declared", result.Environments)
	}
}

// TestPackProjectNarrowsIncludes verifies --project copies only the named
// project's includes.
func TestPackProjectNarrowsIncludes(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{Projects: []string{"web"}, OutDir: out})

	// Only the web project directory is written; api is not selected at all.
	present(t, out, "web/app.yaml")
	absent(t, out, "api/app.yaml")
	absent(t, out, "api/api.yaml")
}

// TestPackPrunesUnselectedProjectsFromManifest verifies a --project selection
// drops the unselected projects from the bundle manifest, so it declares only the
// projects the bundle actually carries.
func TestPackPrunesUnselectedProjectsFromManifest(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{Projects: []string{"web"}, OutDir: out})

	got := manifestProjects(t, filepath.Join(out, "envx.yaml"))
	if !equal(got, []string{"web"}) {
		t.Errorf("bundle manifest projects = %v, want [web]", got)
	}
}

// TestPackDuplicatesSharedNamespace verifies a namespace two projects both
// include is copied into each project's directory rather than shared.
func TestPackDuplicatesSharedNamespace(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	result := mustPack(t, ws, Params{Environments: []string{"development"}, OutDir: out})

	// env/app is included by both api and web, so it lands in both directories.
	present(t, out, "api/app.yaml")
	present(t, out, "web/app.yaml")
	count := 0
	for _, f := range result.Files {
		if strings.HasSuffix(f, "/app.yaml") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("app.yaml copies = %d, want 2 (one per project)", count)
	}
}

// TestPackRejectsUnknownEnvironment verifies an unknown environment fails before
// any file is written.
func TestPackRejectsUnknownEnvironment(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	_, err := Pack(ws, Params{Environments: []string{"ghost"}, OutDir: out})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err = %v, want an unknown-environment error", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Error("output directory was created despite a selection error")
	}
}

// TestPackRejectsUnknownProject verifies an unknown project name is rejected.
func TestPackRejectsUnknownProject(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)

	out := filepath.Join(t.TempDir(), "dist")
	_, err := Pack(ws, Params{Projects: []string{"nope"}, OutDir: out})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v, want an unknown-project error", err)
	}
}

// TestPackRequiresOutDir verifies an empty output directory is rejected.
func TestPackRequiresOutDir(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	if _, err := Pack(ws, Params{}); err == nil {
		t.Fatal("expected an error for an empty output directory")
	}
}

// TestPackFailsOnEmptyInclude verifies an include matching no file on disk is a
// hard error.
func TestPackFailsOnEmptyInclude(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	ws.Projects = append(ws.Projects, Project{
		Name:     "typo",
		Includes: []string{"env/missing"},
	})

	_, err := Pack(ws, Params{OutDir: filepath.Join(t.TempDir(), "dist")})
	if err == nil || !strings.Contains(err.Error(), "env/missing") {
		t.Fatalf("err = %v, want an empty-include error", err)
	}
}

// TestPackWithoutSecretsStore verifies a workspace with no secrets store packs
// successfully and omits secrets.yaml.
func TestPackWithoutSecretsStore(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	if err := os.Remove(ws.SecretsPath); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{Environments: []string{"development"}, OutDir: out})
	absent(t, out, "secrets.yaml")
}

// TestPackFiltersSecrets verifies the bundle's secrets store keeps only the
// referenced value, dropping an unused entry, an unused group, and every public
// key, while preserving the referenced value verbatim.
func TestPackFiltersSecrets(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{Environments: []string{"development"}, OutDir: out})

	got := string(readFile(t, filepath.Join(out, "secrets.yaml")))
	// The referenced value survives with its content intact.
	if !strings.Contains(got, "token: t0k") {
		t.Errorf("referenced secret missing from bundle store:\n%s", got)
	}
	// Everything unreferenced is gone.
	gone := []string{"unused", "nope", "other", "x: y", "public_keys", "PUBKEY"}
	for _, dropped := range gone {
		if strings.Contains(got, dropped) {
			t.Errorf("bundle store still contains %q:\n%s", dropped, got)
		}
	}
}

// TestPackOmitsStoreWhenNothingReferenced verifies a workspace whose selected
// namespaces reference no secret carries no secrets store at all.
func TestPackOmitsStoreWhenNothingReferenced(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development]\n"+
			"projects:\n  app:\n    includes: [env/app]\n")
	writeSource(t, root, "env/app.yaml", "A: base\n") // no secret:// reference
	secrets := writeSource(t, root, "secrets.yaml",
		"secrets:\n  shared:\n    token: t0k\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		SecretsPath:  secrets,
		Environments: []string{"development"},
		Projects:     []Project{{Name: "app", Includes: []string{"env/app"}}},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})
	absent(t, out, "secrets.yaml")
}

// TestPackStandardizesManifestAndStoreNames verifies pack writes the bundle under
// envx's default filenames whatever the source workspace called them, and drops an
// explicit secrets path so the store resolves to secrets.yaml beside the manifest.
func TestPackStandardizesManifestAndStoreNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "config.yaml",
		"environments: [production]\n"+
			"secrets:\n  path: private/vault.yaml\n  keys-path: private/envx.keys\n"+
			"projects:\n  app:\n    includes: [env/app]\n")
	writeSource(t, root, "env/app.yaml", "TOKEN: secret://shared/token\n")
	store := writeSource(t, root, "private/vault.yaml",
		"public_keys:\n  shared: PUBKEY\nsecrets:\n  shared:\n    token: t0k\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		SecretsPath:  store,
		Environments: []string{"production"},
		Projects:     []Project{{Name: "app", Includes: []string{"env/app"}}},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	// The bundle uses the standardized names, not config.yaml / vault.yaml.
	present(t, out, "envx.yaml")
	present(t, out, "secrets.yaml")
	absent(t, out, "config.yaml")
	absent(t, out, "vault.yaml")

	manifestBody := string(readFile(t, filepath.Join(out, "envx.yaml")))
	// The explicit secrets path is gone so the default secrets.yaml resolves; the
	// harmless keys-path is left as declared.
	if strings.Contains(manifestBody, "vault.yaml") {
		t.Errorf("bundle manifest still declares the source secrets path:\n%s", manifestBody)
	}
	if !strings.Contains(manifestBody, "keys-path:") {
		t.Errorf("bundle manifest dropped keys-path unexpectedly:\n%s", manifestBody)
	}
	// The referenced secret survived into the standardized store.
	storeBody := string(readFile(t, filepath.Join(out, "secrets.yaml")))
	if !strings.Contains(storeBody, "token: t0k") {
		t.Errorf("referenced secret missing from standardized store:\n%s", storeBody)
	}
}

// TestPackDropsEmptySecretsBlock verifies that when an explicit path is the only
// setting under secrets, the now-empty secrets block is removed entirely.
func TestPackDropsEmptySecretsBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [production]\n"+
			"secrets:\n  path: private/vault.yaml\n"+
			"projects:\n  app:\n    includes: [env/app]\n")
	writeSource(t, root, "env/app.yaml", "TOKEN: secret://shared/token\n")
	store := writeSource(t, root, "private/vault.yaml",
		"secrets:\n  shared:\n    token: t0k\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		SecretsPath:  store,
		Environments: []string{"production"},
		Projects:     []Project{{Name: "app", Includes: []string{"env/app"}}},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	bundleManifest := string(readFile(t, filepath.Join(out, "envx.yaml")))
	if strings.Contains(bundleManifest, "secrets:") {
		t.Errorf("empty secrets block was not removed:\n%s", bundleManifest)
	}
}

// TestPackRejectsNonEmptyOutDir verifies a non-empty output directory is refused
// by default and the pre-existing content is left untouched.
func TestPackRejectsNonEmptyOutDir(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	stale := writeSource(t, out, "leftover.txt", "keep me\n")

	_, err := Pack(ws, Params{Environments: []string{"production"}, OutDir: out})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("err = %v, want a non-empty-directory error", err)
	}
	if _, statErr := os.Stat(stale); statErr != nil {
		t.Errorf("pre-existing file was disturbed: %v", statErr)
	}
	// No bundle was written alongside the stale file.
	absent(t, out, "envx.yaml")
}

// TestPackRejectsOutPathThatIsAFile verifies an --out that names an existing
// non-directory is refused rather than clobbered.
func TestPackRejectsOutPathThatIsAFile(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := writeSource(t, t.TempDir(), "dist", "i am a file\n")

	_, err := Pack(ws, Params{Environments: []string{"production"}, OutDir: out})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("err = %v, want a not-a-directory error", err)
	}
}

// TestPackAllowsEmptyExistingOutDir verifies an existing but empty output
// directory packs without --force.
func TestPackAllowsEmptyExistingOutDir(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(out, 0o750); err != nil {
		t.Fatal(err)
	}

	mustPack(t, ws, Params{Environments: []string{"production"}, OutDir: out})
	present(t, out, "envx.yaml")
}

// TestPackForceReplacesNonEmptyOutDir verifies --force clears an existing
// non-empty directory so no stale file survives into the new bundle.
func TestPackForceReplacesNonEmptyOutDir(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	writeSource(t, out, "leftover.txt", "delete me\n")

	mustPack(t, ws, Params{Environments: []string{"production"}, OutDir: out, Force: true})

	absent(t, out, "leftover.txt")
	present(t, out, "envx.yaml")
	present(t, out, "api/app.production.yaml")
}

// TestPackForceLeavesExistingBundleOnPrewriteError verifies a --force run that
// fails before writing (an unknown environment) does not clear the destination.
func TestPackForceLeavesExistingBundleOnPrewriteError(t *testing.T) {
	t.Parallel()
	_, ws := newWorkspace(t)
	out := filepath.Join(t.TempDir(), "dist")
	stale := writeSource(t, out, "leftover.txt", "keep me\n")

	_, err := Pack(ws, Params{Environments: []string{"ghost"}, OutDir: out, Force: true})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err = %v, want an unknown-environment error", err)
	}
	if _, statErr := os.Stat(stale); statErr != nil {
		t.Errorf("--force cleared the directory despite a pre-write error: %v", statErr)
	}
}

// TestPackSingleProjectStillNests verifies a single-project pack still nests the
// project's files under a <project>/ directory, so the layout never forks on
// project count.
func TestPackSingleProjectStillNests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development]\n"+
			"projects:\n  solo:\n    includes: [env/app]\n")
	writeSource(t, root, "env/app.yaml", "A: base\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		Environments: []string{"development"},
		Projects:     []Project{{Name: "solo", Includes: []string{"env/app"}}},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	present(t, out, "solo/app.yaml")
	absent(t, out, "app.yaml")
	got := projectIncludes(t, filepath.Join(out, "envx.yaml"), "solo")
	if !equal(got, []string{"solo/app"}) {
		t.Errorf("solo includes = %v, want [solo/app]", got)
	}
}

// TestPackStoreCoversUnionOfProjects verifies the bundle keeps one shared
// secrets.yaml at the root, filtered to the union of every selected project's
// references — a secret referenced by only one of two selected projects survives.
func TestPackStoreCoversUnionOfProjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := writeSource(t, root, "envx.yaml",
		"environments: [development]\n"+
			"projects:\n"+
			"  one:\n    includes: [env/one]\n"+
			"  two:\n    includes: [env/two]\n")
	writeSource(t, root, "env/one.yaml", "A: secret://shared/alpha\n")
	writeSource(t, root, "env/two.yaml", "B: secret://shared/beta\n")
	store := writeSource(t, root, "secrets.yaml",
		"secrets:\n  shared:\n    alpha: a0\n    beta: b0\n    gamma: g0\n")
	ws := Workspace{
		ManifestPath: manifest,
		Root:         root,
		SecretsPath:  store,
		Environments: []string{"development"},
		Projects: []Project{
			{Name: "one", Includes: []string{"env/one"}},
			{Name: "two", Includes: []string{"env/two"}},
		},
	}
	out := filepath.Join(t.TempDir(), "dist")

	mustPack(t, ws, Params{OutDir: out})

	// A single store at the root holds both projects' references but not the unused
	// one.
	present(t, out, "secrets.yaml")
	got := string(readFile(t, filepath.Join(out, "secrets.yaml")))
	if !strings.Contains(got, "alpha: a0") || !strings.Contains(got, "beta: b0") {
		t.Errorf("store missing a referenced secret from the union:\n%s", got)
	}
	if strings.Contains(got, "gamma") {
		t.Errorf("store kept an unreferenced secret:\n%s", got)
	}
}

// equal reports whether two string slices hold the same elements in order.
func equal(a, b []string) bool {
	return slices.Equal(a, b)
}
