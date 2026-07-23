package envmerge

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------

// setupWorkspace creates a temp directory with one namespace (env/postgres) and
// returns its path. envmerge reads only the namespace overlays, so no other
// files are needed.
func setupWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	envDir := filepath.Join(dir, "env")
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(envDir, "postgres.yaml"),
		[]byte("host: localhost\nport: 5432\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(envDir, "postgres.production.yaml"),
		[]byte("host: prod-db\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	return dir
}

// -------------------------------------------------------------------------------------

// baseParams builds envmerge.Params for the temp workspace declaring the
// development and production environments.
func baseParams(dir string) Params {
	return Params{
		Includes:     []string{filepath.Join(dir, "env", "postgres")},
		Environments: []string{"development", "production"},
	}
}

// -------------------------------------------------------------------------------------

// TestResolveSuccess verifies params resolve to the merged environment.
func TestResolveSuccess(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.Settings = Settings{Env: "development"}
	res, err := Build(p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (development overlay absent)", v)
	}
}

// -------------------------------------------------------------------------------------

// TestResolveDefaultEnv verifies an empty Settings.Env falls back to the first
// declared environment.
func TestResolveDefaultEnv(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	res, err := Build(p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := p.Environments[0]
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (default env %q)", v, want)
	}
}

// -------------------------------------------------------------------------------------

// TestResolveOverride verifies Settings.Env selects that environment's overlay
// (as diff relies on, passing each side).
func TestResolveOverride(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.Settings = Settings{Env: "production"}
	res, err := Build(p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "prod-db" {
		t.Errorf("HOST = %q, want prod-db", v)
	}
}

// -------------------------------------------------------------------------------------

// TestResolveErrors verifies an undeclared environment fails.
func TestResolveErrors(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.Settings = Settings{Env: "nope"}
	if _, err := Build(p); err == nil {
		t.Error("expected error for undeclared environment")
	}
}

// -------------------------------------------------------------------------------------

// writeYAML writes a YAML file into dir.
func writeYAML(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesOverlay verifies an overlay overrides the base and origin
// tracking attributes the value to the overlay file.
func TestMergeNamespacesOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\nport: 5432\n")
	writeYAML(t, dir, "postgres.development.yaml", "host: dev-db.local\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}}, Settings{Env: "development"},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}

	if v, _ := res.Get("HOST"); v != "dev-db.local" {
		t.Errorf("HOST = %q, want dev-db.local", v)
	}
	if v, _ := res.Get("PORT"); v != "5432" {
		t.Errorf("PORT = %q, want 5432", v)
	}
	origin, ok := res.Origin("HOST")
	if !ok {
		t.Fatal("expected origin for HOST")
	}
	if filepath.Base(origin.Winner.File) != "postgres.development.yaml" {
		t.Errorf("HOST winner = %q, want overlay", origin.Winner.File)
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesShadowTracksBase verifies that when an environment overlay
// overrides a base value, origin tracking records the base file as shadowed by
// the winning overlay within the same namespace, while a key defined only in the
// base has no shadowed sources.
func TestMergeNamespacesShadowTracksBase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\nport: 5432\n")
	writeYAML(t, dir, "postgres.production.yaml", "host: prod-db\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}}, Settings{Env: "production"},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}

	origin, ok := res.Origin("HOST")
	if !ok {
		t.Fatal("expected origin for HOST")
	}
	if filepath.Base(origin.Winner.File) != "postgres.production.yaml" {
		t.Errorf("HOST winner = %q, want overlay", origin.Winner.File)
	}
	if len(origin.Shadowed) != 1 {
		t.Fatalf("HOST shadowed = %v, want exactly the base file", origin.Shadowed)
	}
	if filepath.Base(origin.Shadowed[0].File) != "postgres.yaml" {
		t.Errorf(
			"HOST shadowed = %q, want base postgres.yaml", origin.Shadowed[0].File,
		)
	}

	// PORT lives only in the base file, so it has no shadowed sources.
	port, ok := res.Origin("PORT")
	if !ok {
		t.Fatal("expected origin for PORT")
	}
	if len(port.Shadowed) != 0 {
		t.Errorf("PORT shadowed = %v, want none", port.Shadowed)
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesRequireOverlaysMissingOverlay verifies require_overlays mode
// errors when an overlay file is absent, while lax mode tolerates it.
func TestMergeNamespacesRequireOverlaysMissingOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	ns := []namespace{{dir: dir, name: "postgres"}}
	_, err := mergeNamespaces(ns, Settings{Env: "production", RequireOverlays: true})
	if err == nil {
		t.Error("expected require_overlays error for missing overlay")
	}
	if _, err := mergeNamespaces(ns, Settings{Env: "production"}); err != nil {
		t.Errorf("lax mode should tolerate missing overlay, got %v", err)
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesNestedFlatEquivalence verifies an overlay may override a
// base value written in the other spelling: a nested log.level in the base is
// overridden by a flat log_level in the overlay, since both collapse to
// LOG_LEVEL. Origin tracking attributes the winner to the overlay and records
// the base as shadowed.
func TestMergeNamespacesNestedFlatEquivalence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "log:\n  level: info\n")
	writeYAML(t, dir, "app.production.yaml", "log_level: warn\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}}, Settings{Env: "production"},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}

	if v, _ := res.Get("LOG_LEVEL"); v != "warn" {
		t.Errorf("LOG_LEVEL = %q, want warn (flat overlay overrides nested base)", v)
	}
	origin, ok := res.Origin("LOG_LEVEL")
	if !ok {
		t.Fatal("expected origin for LOG_LEVEL")
	}
	if filepath.Base(origin.Winner.File) != "app.production.yaml" {
		t.Errorf("LOG_LEVEL winner = %q, want overlay", origin.Winner.File)
	}
	if origin.Winner.Key != "log_level" {
		t.Errorf("LOG_LEVEL winner key = %q, want log_level", origin.Winner.Key)
	}
	if len(origin.Shadowed) != 1 || origin.Shadowed[0].Key != "log.level" {
		t.Errorf("LOG_LEVEL shadowed = %v, want base log.level", origin.Shadowed)
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesNestedPartialOverride verifies layered flattening preserves
// deep-merge semantics for nested maps: an overlay that sets one leaf under a
// mapping overrides only that leaf, leaving the base's sibling leaves intact.
func TestMergeNamespacesNestedPartialOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "log:\n  level: info\n  format: json\n")
	writeYAML(t, dir, "app.production.yaml", "log:\n  level: warn\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}}, Settings{Env: "production"},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}

	if v, _ := res.Get("LOG_LEVEL"); v != "warn" {
		t.Errorf("LOG_LEVEL = %q, want warn", v)
	}
	if v, _ := res.Get("LOG_FORMAT"); v != "json" {
		t.Errorf("LOG_FORMAT = %q, want json (base leaf preserved)", v)
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesSingleFileCollision verifies two spellings that collapse to
// the same env key within a single file still error, since the value would
// otherwise be ambiguous.
func TestMergeNamespacesSingleFileCollision(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "log:\n  level: info\nlog_level: warn\n")

	_, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}}, Settings{Env: "development"},
	)
	if err == nil {
		t.Fatal("expected collision error for two spellings in one file")
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesPrefixSuffix verifies global prefix/suffix and namespace
// prefixing apply to every key.
func TestMergeNamespacesPrefixSuffix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}},
		Settings{Env: "development", Prefix: "app", Suffix: "v2", NamespacePrefix: true},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if _, ok := res.Get("APP_POSTGRES_HOST_V2"); !ok {
		t.Errorf("expected APP_POSTGRES_HOST_V2, got keys %v", res.Keys())
	}
}

// -------------------------------------------------------------------------------------

// TestMergeNamespacesJoinsList verifies a list-valued leaf is joined into a
// single env var using the configured delimiter.
func TestMergeNamespacesJoinsList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "hosts:\n  - a\n  - b\n  - c\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}},
		Settings{Env: "development", Delimiter: "|"},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if v, _ := res.Get("HOSTS"); v != "a|b|c" {
		t.Errorf("HOSTS = %q, want a|b|c", v)
	}
}

// -------------------------------------------------------------------------------------

// TestBuildJoinsListWithDefaultDelimiter verifies Build applies the default comma
// delimiter to a list leaf when none is configured.
func TestBuildJoinsListWithDefaultDelimiter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "hosts:\n  - a\n  - b\n")

	res, err := Build(Params{
		Includes:     []string{filepath.Join(dir, "app")},
		Environments: []string{"development"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if v, _ := res.Get("HOSTS"); v != "a,b" {
		t.Errorf("HOSTS = %q, want a,b (default comma)", v)
	}
}
