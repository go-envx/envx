package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
)

// -------------------------------------------------------------------------------------
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

// -------------------------------------------------------------------------------------
// TestLoadValid verifies a well-formed manifest parses with its path recorded.
func TestLoadValid(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("manifest/valid-secrets")
	m, got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != path {
		t.Errorf("path = %q, want %q", got, path)
	}
	if !m.HasEnvironment("production") {
		t.Error("expected production environment to be present")
	}
	if m.Secrets.SecretsPath != "./private/secrets.yaml" {
		t.Errorf("SecretsPath = %q, want ./private/secrets.yaml", m.Secrets.SecretsPath)
	}
	if _, ok := m.LookupProject("api"); !ok {
		t.Error("expected project api to be present")
	}
}

// -------------------------------------------------------------------------------------
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
			if _, _, err := Load(writeManifest(t, body)); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestLoadDiscovers verifies Load discovers an explicit path and loads it.
func TestLoadDiscovers(t *testing.T) {
	t.Parallel()

	m, _, err := Load(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := m.LookupProject("api-core"); !ok {
		t.Error("expected project api-core to be present")
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverExplicit verifies an explicit path is honored and a missing path
// is an error.
func TestDiscoverExplicit(t *testing.T) {
	t.Parallel()

	got, err := discover(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}

	if _, err := discover(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("expected error for missing explicit path")
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverWalkUp verifies that with no explicit path the search walks up from
// the working directory to the nearest envx.yaml.
func TestDiscoverWalkUp(t *testing.T) {
	// t.Chdir forbids t.Parallel.
	man := fixtures.Manifest("basic")
	t.Chdir(filepath.Join(filepath.Dir(man), "apps", "api-core", "env"))

	got, err := discover("")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != man {
		t.Errorf("Discover() = %q, want %q", got, man)
	}
}
