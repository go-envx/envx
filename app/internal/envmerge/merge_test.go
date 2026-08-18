package envmerge

import (
	"path/filepath"
	"testing"
)

// buildNamespace merges a single namespace for one environment through the shared
// kernel, declaring development and production so either environment validates. It
// exercises the merge/provenance behavior a Manager operation relies on.
func buildNamespace(
	t *testing.T, dir, name, env string, settings Settings,
) (*Environment, error) {
	t.Helper()
	return mergeEnv(t, Params{
		Includes:           []string{filepath.Join(dir, name)},
		Environments:       []string{"development", "production"},
		DefaultEnvironment: env,
		Settings:           settings,
	})
}

// TestMergeNamespacesOverlay verifies an overlay overrides the base and origin
// tracking attributes the value to the overlay file.
func TestMergeNamespacesOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\nport: 5432\n")
	writeYAML(t, dir, "postgres.development.yaml", "host: dev-db.local\n")

	res, err := buildNamespace(t, dir, "postgres", "development", Settings{})
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

// TestMergeNamespacesShadowTracksBase verifies that when an environment overlay
// overrides a base value, origin tracking records the base file as shadowed by
// the winning overlay within the same namespace, while a key defined only in the
// base has no shadowed sources.
func TestMergeNamespacesShadowTracksBase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\nport: 5432\n")
	writeYAML(t, dir, "postgres.production.yaml", "host: prod-db\n")

	res, err := buildNamespace(t, dir, "postgres", "production", Settings{})
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

// TestMergeNamespacesRequireOverlaysMissingOverlay verifies require_overlays mode
// errors when an overlay file is absent, while lax mode tolerates it.
func TestMergeNamespacesRequireOverlaysMissingOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	_, err := buildNamespace(
		t, dir, "postgres", "production", Settings{RequireOverlays: true},
	)
	if err == nil {
		t.Error("expected require_overlays error for missing overlay")
	}
	if _, err := buildNamespace(t, dir, "postgres", "production", Settings{}); err != nil {
		t.Errorf("lax mode should tolerate missing overlay, got %v", err)
	}
}

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

	res, err := buildNamespace(t, dir, "app", "production", Settings{})
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

// TestMergeNamespacesNestedPartialOverride verifies layered flattening preserves
// deep-merge semantics for nested maps: an overlay that sets one leaf under a
// mapping overrides only that leaf, leaving the base's sibling leaves intact.
func TestMergeNamespacesNestedPartialOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "log:\n  level: info\n  format: json\n")
	writeYAML(t, dir, "app.production.yaml", "log:\n  level: warn\n")

	res, err := buildNamespace(t, dir, "app", "production", Settings{})
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

// TestMergeNamespacesSingleFileCollision verifies two spellings that collapse to
// the same env key within a single file still error, since the value would
// otherwise be ambiguous.
func TestMergeNamespacesSingleFileCollision(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "log:\n  level: info\nlog_level: warn\n")

	_, err := buildNamespace(t, dir, "app", "development", Settings{})
	if err == nil {
		t.Fatal("expected collision error for two spellings in one file")
	}
}

// TestMergeNamespacesPrefixSuffix verifies global prefix/suffix and namespace
// prefixing apply to every key.
func TestMergeNamespacesPrefixSuffix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	res, err := buildNamespace(
		t, dir, "postgres", "development",
		Settings{Prefix: "app", Suffix: "v2", NamespacePrefix: true},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if _, ok := res.Get("APP_POSTGRES_HOST_V2"); !ok {
		t.Errorf("expected APP_POSTGRES_HOST_V2, got keys %v", res.Keys())
	}
}

// TestMergeNamespacesJoinsList verifies a list-valued leaf is joined into a
// single env var using the configured delimiter.
func TestMergeNamespacesJoinsList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "hosts:\n  - a\n  - b\n  - c\n")

	res, err := buildNamespace(
		t, dir, "app", "development", Settings{Delimiter: "|"},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if v, _ := res.Get("HOSTS"); v != "a|b|c" {
		t.Errorf("HOSTS = %q, want a|b|c", v)
	}
}
