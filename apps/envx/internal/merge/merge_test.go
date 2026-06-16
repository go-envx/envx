package merge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/file"
)

// -------------------------------------------------------------------------------------
// TestResolveBasicMerge verifies that base and environment overlay files are
// deep-merged correctly for a single namespace.
func TestResolveBasicMerge(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "postgres.yaml", `
		host: localhost
		port: 5432
		credentials:
		  username: dev_user
		  password: dev_pass
	`)
	file.Write(t, dir, "postgres.development.yaml", `
		credentials:
		  password: dev_override
	`)

	ns := []Namespace{{Dir: dir, Name: "postgres"}}
	result, err := Resolve(ns, "development", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEnv(t, result, "HOST", "localhost")
	assertEnv(t, result, "PORT", "5432")
	assertEnv(t, result, "CREDENTIALS_USERNAME", "dev_user")
	assertEnv(t, result, "CREDENTIALS_PASSWORD", "dev_override")
}

// -------------------------------------------------------------------------------------
// TestResolveMultipleNamespaces verifies last-wins ordering when multiple
// namespaces produce overlapping keys, and checks provenance tracking.
func TestResolveMultipleNamespaces(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// First namespace: postgres
	pgDir := filepath.Join(dir, "postgres")
	if err := os.MkdirAll(pgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	file.Write(t, pgDir, "postgres.yaml", `
		host: pg-host
		port: 5432
	`)
	file.Write(t, pgDir, "postgres.development.yaml", `
		host: localhost
	`)

	// Second namespace: app (loads last, wins)
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o750); err != nil {
		t.Fatal(err)
	}
	file.Write(t, appDir, "app.yaml", `
		port: 3000
		host: app-host
	`)

	ns := []Namespace{
		{Dir: pgDir, Name: "postgres"},
		{Dir: appDir, Name: "app"},
	}
	result, err := Resolve(ns, "development", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// app's HOST wins over postgres's HOST (last-wins).
	assertEnv(t, result, "HOST", "app-host")
	// app's PORT wins over postgres's PORT.
	assertEnv(t, result, "PORT", "3000")

	// Check provenance: HOST was overridden.
	prov := result.Provenance["HOST"]
	if prov == nil {
		t.Fatal("expected provenance for HOST")
	}
	if len(prov.Overridden) != 1 {
		t.Errorf("expected 1 overridden source, got %d", len(prov.Overridden))
	}
}

// -------------------------------------------------------------------------------------
// TestResolveMissingBaseFileErrors verifies that a missing base YAML file
// produces an error.
func TestResolveMissingBaseFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ns := []Namespace{{Dir: dir, Name: "nonexistent"}}

	_, err := Resolve(ns, "development", Options{})
	if err == nil {
		t.Fatal("expected error for missing base file")
	}
}

// -------------------------------------------------------------------------------------
// TestResolveMissingEnvFileOptional verifies that a missing environment
// overlay is silently skipped in non-strict mode.
func TestResolveMissingEnvFileOptional(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "app.yaml", `key: value`)
	// No app.development.yaml — should be fine without strict mode.

	ns := []Namespace{{Dir: dir, Name: "app"}}
	result, err := Resolve(ns, "development", Options{Strict: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEnv(t, result, "KEY", "value")
}

// -------------------------------------------------------------------------------------
// TestResolveMissingEnvFileStrictErrors verifies that a missing environment
// overlay produces an error in strict mode.
func TestResolveMissingEnvFileStrictErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "app.yaml", `key: value`)
	// No app.development.yaml — strict mode should error.

	ns := []Namespace{{Dir: dir, Name: "app"}}
	_, err := Resolve(ns, "development", Options{Strict: true})
	if err == nil {
		t.Fatal("expected error for missing env file in strict mode")
	}
}

// -------------------------------------------------------------------------------------
// TestDeepMerge verifies recursive map merging: scalar override, new keys,
// nested maps, list replacement, and nil source handling.
func TestDeepMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dst  map[string]any
		src  map[string]any
		want map[string]any
	}{
		{
			name: "scalar override",
			dst:  map[string]any{"a": "1"},
			src:  map[string]any{"a": "2"},
			want: map[string]any{"a": "2"},
		},
		{
			name: "add new key",
			dst:  map[string]any{"a": "1"},
			src:  map[string]any{"b": "2"},
			want: map[string]any{"a": "1", "b": "2"},
		},
		{
			name: "recursive merge",
			dst:  map[string]any{"db": map[string]any{"host": "h", "port": "5432"}},
			src:  map[string]any{"db": map[string]any{"host": "new-h"}},
			want: map[string]any{"db": map[string]any{"host": "new-h", "port": "5432"}},
		},
		{
			name: "list replaces",
			dst:  map[string]any{"tags": []any{"a", "b"}},
			src:  map[string]any{"tags": []any{"c"}},
			want: map[string]any{"tags": []any{"c"}},
		},
		{
			name: "nil src",
			dst:  map[string]any{"a": "1"},
			src:  nil,
			want: map[string]any{"a": "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := deepMerge(tt.dst, tt.src)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestFlattenCollisionDetection verifies that flatten detects and reports
// when two different paths produce the same env var key.
func TestFlattenCollisionDetection(t *testing.T) {
	t.Parallel()

	// "api_key" at top level and "api.key" nested both produce API_KEY.
	m := map[string]any{
		"api_key": "val1",
		"api":     map[string]any{"key": "val2"},
	}

	_, err := flatten(m)
	if err == nil {
		t.Fatal("expected flatten collision error")
	}
}

// -------------------------------------------------------------------------------------
// TestFlattenNestedKeys verifies that nested YAML maps are flattened to
// uppercase underscore-separated env var keys.
func TestFlattenNestedKeys(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"postgres": map[string]any{
			"username": "admin",
			"host":     "localhost",
		},
		"port": 5432,
	}

	flat, err := flatten(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if flat["POSTGRES_USERNAME"] != "admin" {
		t.Errorf("POSTGRES_USERNAME = %q, want %q", flat["POSTGRES_USERNAME"], "admin")
	}
	if flat["POSTGRES_HOST"] != "localhost" {
		t.Errorf("POSTGRES_HOST = %q, want %q", flat["POSTGRES_HOST"], "localhost")
	}
	if flat["PORT"] != "5432" {
		t.Errorf("PORT = %q, want %q", flat["PORT"], "5432")
	}
}

func assertEnv(t *testing.T, r *Result, key, want string) {
	t.Helper()
	got, ok := r.Env[key]
	if !ok {
		t.Errorf("key %q not found in result", key)
		return
	}
	if got != want {
		t.Errorf("Env[%q] = %q, want %q", key, got, want)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveNamespacePrefix verifies that namespace prefixing prepends the
// namespace name to all env var keys.
func TestResolveNamespacePrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "postgres.yaml", `
		host: localhost
		port: 5432
	`)

	ns := []Namespace{{Dir: dir, Name: "postgres"}}
	result, err := Resolve(ns, "development", Options{NamespacePrefix: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEnv(t, result, "POSTGRES_HOST", "localhost")
	assertEnv(t, result, "POSTGRES_PORT", "5432")

	// Ensure unprefixed keys do NOT exist.
	if _, ok := result.Env["HOST"]; ok {
		t.Error("expected HOST to not exist (should be POSTGRES_HOST)")
	}
}

// -------------------------------------------------------------------------------------
// TestResolveNamespacePrefixDisabled verifies that disabling namespace
// prefixing produces raw keys without the namespace name.
func TestResolveNamespacePrefixDisabled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "postgres.yaml", `
		host: localhost
		port: 5432
	`)

	ns := []Namespace{{Dir: dir, Name: "postgres"}}
	result, err := Resolve(ns, "development", Options{NamespacePrefix: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEnv(t, result, "HOST", "localhost")
	assertEnv(t, result, "PORT", "5432")
}

// -------------------------------------------------------------------------------------
// TestResolveNamespacePrefixHyphenated verifies that hyphenated namespace
// names are converted to underscored uppercase prefixes.
func TestResolveNamespacePrefixHyphenated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "api-core.yaml", `
		app_name: api-core
		port: 3000
	`)

	ns := []Namespace{{Dir: dir, Name: "api-core"}}
	result, err := Resolve(ns, "development", Options{NamespacePrefix: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEnv(t, result, "API_CORE_APP_NAME", "api-core")
	assertEnv(t, result, "API_CORE_PORT", "3000")
}

// -------------------------------------------------------------------------------------
// TestResolvePrefixSuffix verifies that global prefix and suffix are applied
// to all env var keys.
func TestResolvePrefixSuffix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "app.yaml", `
		host: localhost
		port: 3000
	`)

	ns := []Namespace{{Dir: dir, Name: "app"}}
	result, err := Resolve(ns, "development", Options{Prefix: "NUXT", Suffix: "V2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEnv(t, result, "NUXT_HOST_V2", "localhost")
	assertEnv(t, result, "NUXT_PORT_V2", "3000")
}

// -------------------------------------------------------------------------------------
// TestResolvePrefixWithNamespacePrefix verifies that both namespace prefix
// and global prefix are applied correctly (namespace first, then global).
func TestResolvePrefixWithNamespacePrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "postgres.yaml", `
		host: localhost
		port: 5432
	`)

	ns := []Namespace{{Dir: dir, Name: "postgres"}}
	result, err := Resolve(ns, "development", Options{
		NamespacePrefix: true,
		Prefix:          "NUXT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Namespace prefix applied first, then global prefix wraps it.
	assertEnv(t, result, "NUXT_POSTGRES_HOST", "localhost")
	assertEnv(t, result, "NUXT_POSTGRES_PORT", "5432")
}
