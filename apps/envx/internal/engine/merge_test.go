package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------
// TestDeepMerge verifies recursive map merging with scalar/list replacement.
func TestDeepMerge(t *testing.T) {
	t.Parallel()

	dst := map[string]any{
		"a": "1",
		"nested": map[string]any{
			"x": "base",
			"y": "keep",
		},
	}
	src := map[string]any{
		"b": "2",
		"nested": map[string]any{
			"x": "override",
		},
	}

	got := deepMerge(dst, src)
	if got["a"] != "1" || got["b"] != "2" {
		t.Errorf("top-level keys wrong: %v", got)
	}
	nested, _ := toMap(got["nested"])
	if nested["x"] != "override" {
		t.Errorf("nested.x = %v, want override", nested["x"])
	}
	if nested["y"] != "keep" {
		t.Errorf("nested.y = %v, want keep", nested["y"])
	}
}

// -------------------------------------------------------------------------------------
// TestFlatten verifies nested-to-env flattening and key normalization.
func TestFlatten(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"host": "localhost",
		"credentials": map[string]any{
			"user-name": "postgres",
		},
	}
	got, err := flatten(in)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	if got["HOST"] != "localhost" {
		t.Errorf("HOST = %q, want localhost", got["HOST"])
	}
	if got["CREDENTIALS_USER_NAME"] != "postgres" {
		t.Errorf("CREDENTIALS_USER_NAME = %q, want postgres", got["CREDENTIALS_USER_NAME"])
	}
}

// -------------------------------------------------------------------------------------
// TestFlattenCollision verifies two paths collapsing to the same key error out.
func TestFlattenCollision(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"api_key": "a",
		"api": map[string]any{
			"key": "b",
		},
	}
	if _, err := flatten(in); err == nil {
		t.Fatal("expected flatten collision error")
	}
}

// -------------------------------------------------------------------------------------
// TestToEnvKey verifies dotted/hyphenated paths normalize to upper-snake.
func TestToEnvKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"postgres.user-name": "POSTGRES_USER_NAME",
		"host":               "HOST",
		"a.b.c":              "A_B_C",
	}
	for in, want := range cases {
		if got := toEnvKey(in); got != want {
			t.Errorf("toEnvKey(%q) = %q, want %q", in, got, want)
		}
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
		[]namespace{{dir: dir, name: "postgres"}}, "development", mergeOptions{},
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
// TestMergeNamespacesStrictMissingOverlay verifies strict mode errors when an
// overlay file is absent, while lax mode tolerates it.
func TestMergeNamespacesStrictMissingOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	ns := []namespace{{dir: dir, name: "postgres"}}
	_, err := mergeNamespaces(ns, "production", mergeOptions{strict: true})
	if err == nil {
		t.Error("expected strict error for missing overlay")
	}
	if _, err := mergeNamespaces(ns, "production", mergeOptions{}); err != nil {
		t.Errorf("lax mode should tolerate missing overlay, got %v", err)
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
		[]namespace{{dir: dir, name: "postgres"}}, "development",
		mergeOptions{prefix: "app", suffix: "v2", namespacePrefix: true},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if _, ok := res.Get("APP_POSTGRES_HOST_V2"); !ok {
		t.Errorf("expected APP_POSTGRES_HOST_V2, got keys %v", res.Keys())
	}
}

// -------------------------------------------------------------------------------------
// TestResultAllIsCopy verifies All returns a defensive copy.
func TestResultAllIsCopy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")
	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}}, "development", mergeOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	all := res.All()
	all["HOST"] = "mutated"
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("internal map was mutated: HOST = %q", v)
	}
}
