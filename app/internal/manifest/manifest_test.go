package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
)

// writeManifest writes a manifest file into a fresh temp dir and returns its
// path.
func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "envx.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// newManager constructs a manager for path using the conventional filename.
func newManager(t *testing.T, path string) *Manager {
	t.Helper()
	m, err := New(Params{Path: path, Filename: "envx.yaml"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

// TestNewRequiresFilename verifies construction fails without a discovery
// filename.
func TestNewRequiresFilename(t *testing.T) {
	t.Parallel()

	if _, err := New(Params{Path: "envx.yaml"}); err == nil {
		t.Error("expected error for empty filename")
	}
}

// TestLoadValid verifies a well-formed manifest parses with its path recorded.
func TestLoadValid(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("manifest/valid-secrets")
	loaded, err := newManager(t, path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Path != path {
		t.Errorf("path = %q, want %q", loaded.Path, path)
	}
	m := loaded.Content
	if !m.HasEnvironment("production") {
		t.Error("expected production environment to be present")
	}
	if m.Secrets.SecretsPath != "./private/secrets.yaml" {
		t.Errorf("SecretsPath = %q, want ./private/secrets.yaml", m.Secrets.SecretsPath)
	}
	if m.Secrets.KeysPath != "./private/envx.keys" {
		t.Errorf("KeysPath = %q, want ./private/envx.keys", m.Secrets.KeysPath)
	}
	if m.Secrets.Cipher != "age" {
		t.Errorf("Cipher = %q, want age", m.Secrets.Cipher)
	}
	if _, ok := m.LookupProject("api"); !ok {
		t.Error("expected project api to be present")
	}
}

// TestLoadDetectsIndent verifies Load reports the source document's block
// indentation and defaults to two spaces when none is detectable.
func TestLoadDetectsIndent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body string
		want int
	}{
		"two spaces": {
			body: "environments:\n  - development\n" +
				"projects:\n  api:\n    includes:\n      - env/x\n",
			want: 2,
		},
		"four spaces": {
			body: "environments:\n    - development\n" +
				"projects:\n    api:\n        includes:\n            - env/x\n",
			want: 4,
		},
		"flow style defaults to two": {
			body: "environments: [development]\n" +
				"projects: {api: {includes: [env/x]}}\n",
			want: 2,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			loaded, err := newManager(t, writeManifest(t, tc.body)).Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if loaded.Indent != tc.want {
				t.Errorf("Indent = %d, want %d", loaded.Indent, tc.want)
			}
		})
	}
}

// TestExistsMissing verifies an absent manifest is reported as not found without
// becoming an error.
func TestExistsMissing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.yaml")
	exists, err := newManager(t, path).Exists()
	if err != nil {
		t.Fatalf("Exists(): %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}
}

// TestExistsPresent verifies a discoverable manifest is reported as present.
func TestExistsPresent(t *testing.T) {
	t.Parallel()

	exists, err := newManager(t, fixtures.Manifest("basic")).Exists()
	if err != nil {
		t.Fatalf("Exists(): %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}
}

// TestLoadInvalid verifies structural validation rejects malformed manifests.
func TestLoadInvalid(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"no environments": "projects:\n  api:\n    includes: [env/x]\n",
		"no projects":     "environments: [development]\n",
		"empty include": "environments: [development]\n" +
			"projects:\n  api:\n    includes: [\"\"]\n",
		"no includes": "environments: [development]\n" +
			"projects:\n  api:\n    includes: []\n",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := newManager(t, writeManifest(t, body)).Load(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// TestLoadDiscovers verifies Load discovers an explicit path and loads it.
func TestLoadDiscovers(t *testing.T) {
	t.Parallel()

	loaded, err := newManager(t, fixtures.Manifest("basic")).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := loaded.Content.LookupProject("api-core"); !ok {
		t.Error("expected project api-core to be present")
	}
}

// TestDiscoverExplicit verifies an explicit path is honored and a missing path
// is an error.
func TestDiscoverExplicit(t *testing.T) {
	t.Parallel()

	got, err := newManager(t, fixtures.Manifest("basic")).discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}

	missing := filepath.Join(t.TempDir(), "missing.yaml")
	if _, err := newManager(t, missing).discover(); err == nil {
		t.Error("expected error for missing explicit path")
	}
}

// TestDiscoverWalkUp verifies that with no explicit path the search walks up from
// the working directory to the nearest envx.yaml.
func TestDiscoverWalkUp(t *testing.T) {
	// t.Chdir forbids t.Parallel.
	man := fixtures.Manifest("basic")
	t.Chdir(filepath.Join(filepath.Dir(man), "apps", "api-core", "env"))

	got, err := newManager(t, "").discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != man {
		t.Errorf("Discover() = %q, want %q", got, man)
	}
}
