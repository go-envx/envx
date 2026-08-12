package envmerge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// baseParams builds envmerge.Params for the temp workspace declaring the
// development and production environments.
func baseParams(dir string) Params {
	return Params{
		Includes:     []string{filepath.Join(dir, "env", "postgres")},
		Environments: []string{"development", "production"},
	}
}

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

// TestResolveErrors verifies an undeclared environment fails.
func TestResolveErrors(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.Settings = Settings{Env: "nope"}
	if _, err := Build(p); err == nil {
		t.Error("expected error for undeclared environment")
	}
}

// writeYAML writes a YAML file into dir.
func writeYAML(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestMergeNamespacesOverlay verifies an overlay overrides the base and origin
// tracking attributes the value to the overlay file.
func TestMergeNamespacesOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\nport: 5432\n")
	writeYAML(t, dir, "postgres.development.yaml", "host: dev-db.local\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}}, Settings{Env: "development"}, nil,
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
		[]namespace{{dir: dir, name: "postgres"}}, Settings{Env: "production"}, nil,
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

// TestMergeNamespacesRequireOverlaysMissingOverlay verifies require_overlays mode
// errors when an overlay file is absent, while lax mode tolerates it.
func TestMergeNamespacesRequireOverlaysMissingOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	ns := []namespace{{dir: dir, name: "postgres"}}
	_, err := mergeNamespaces(ns, Settings{Env: "production", RequireOverlays: true}, nil)
	if err == nil {
		t.Error("expected require_overlays error for missing overlay")
	}
	if _, err := mergeNamespaces(ns, Settings{Env: "production"}, nil); err != nil {
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

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}}, Settings{Env: "production"}, nil,
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

// TestMergeNamespacesNestedPartialOverride verifies layered flattening preserves
// deep-merge semantics for nested maps: an overlay that sets one leaf under a
// mapping overrides only that leaf, leaving the base's sibling leaves intact.
func TestMergeNamespacesNestedPartialOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "log:\n  level: info\n  format: json\n")
	writeYAML(t, dir, "app.production.yaml", "log:\n  level: warn\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}}, Settings{Env: "production"}, nil,
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

// TestMergeNamespacesSingleFileCollision verifies two spellings that collapse to
// the same env key within a single file still error, since the value would
// otherwise be ambiguous.
func TestMergeNamespacesSingleFileCollision(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "log:\n  level: info\nlog_level: warn\n")

	_, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}}, Settings{Env: "development"}, nil,
	)
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

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}},
		Settings{Env: "development", Prefix: "app", Suffix: "v2", NamespacePrefix: true},
		nil,
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

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "app"}},
		Settings{Env: "development", Delimiter: "|"},
		nil,
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if v, _ := res.Get("HOSTS"); v != "a|b|c" {
		t.Errorf("HOSTS = %q, want a|b|c", v)
	}
}

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

// fakeResolver implements Resolver for testing the reference-resolution step: it
// maps known reference values to results, fails a designated value, and passes
// everything else through unchanged.
type fakeResolver struct {
	values map[string]string
	fail   string
}

// Resolve maps value to its result, erroring on the designated failure value and
// returning unknown values unchanged.
func (f fakeResolver) Resolve(value, _ string) (string, error) {
	if f.fail != "" && value == f.fail {
		return "", errors.New("resolve failed")
	}
	if v, ok := f.values[value]; ok {
		return v, nil
	}
	return value, nil
}

// TestBuildResolvesReferences verifies Build routes each merged value through the
// resolver, dereferencing references while leaving plain values untouched.
func TestBuildResolvesReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "password: secret://x\nplain: keep\n")

	res, err := Build(Params{
		Includes:      []string{filepath.Join(dir, "app")},
		Environments:  []string{"development"},
		ValueResolver: fakeResolver{values: map[string]string{"secret://x": "resolved-pw"}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if v, _ := res.Get("PASSWORD"); v != "resolved-pw" {
		t.Errorf("PASSWORD = %q, want resolved-pw", v)
	}
	if v, _ := res.Get("PLAIN"); v != "keep" {
		t.Errorf("PLAIN = %q, want keep (passthrough)", v)
	}
}

// TestBuildResolverError verifies a resolver failure is deferred per key: Build
// still succeeds, but Verify and the failing key's Err surface the error while
// unrelated keys resolve normally.
func TestBuildResolverError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "password: secret://boom\nplain: keep\n")

	res, err := Build(Params{
		Includes:      []string{filepath.Join(dir, "app")},
		Environments:  []string{"development"},
		ValueResolver: fakeResolver{fail: "secret://boom"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := res.Err("PASSWORD"); err == nil {
		t.Error("expected a deferred error for the failing key")
	}
	if err := res.Verify(); err == nil {
		t.Error("expected Verify to surface the deferred resolver failure")
	}
	if _, ok := res.Get("PASSWORD"); ok {
		t.Error("a failed key should not report a resolved value")
	}
	if v, _ := res.Get("PLAIN"); v != "keep" {
		t.Errorf("PLAIN = %q, want keep (unrelated key still resolves)", v)
	}
}

// TestBuildToleratesUnrelatedDanglingReferences verifies a dangling reference
// behind one key never blocks another key: the resolved key is available, the
// failed key carries its own error, and Verify names the failing key.
func TestBuildToleratesUnrelatedDanglingReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "good: secret://ok\nbad: secret://missing\n")

	res, err := Build(Params{
		Includes:     []string{filepath.Join(dir, "app")},
		Environments: []string{"development"},
		ValueResolver: fakeResolver{
			values: map[string]string{"secret://ok": "resolved"},
			fail:   "secret://missing",
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if v, ok := res.Get("GOOD"); !ok || v != "resolved" {
		t.Errorf("GOOD = %q (ok=%v), want resolved", v, ok)
	}
	if res.Err("GOOD") != nil {
		t.Errorf("GOOD carries an unexpected error: %v", res.Err("GOOD"))
	}
	if res.Err("BAD") == nil {
		t.Error("BAD should carry its own resolution error")
	}
	verifyErr := res.Verify()
	if verifyErr == nil {
		t.Fatal("expected Verify to fail on the dangling reference")
	}
	if !strings.Contains(verifyErr.Error(), "BAD") {
		t.Errorf("Verify error = %v, want it to name the failing key BAD", verifyErr)
	}
}

// TestBuildSkipsShadowedReferences verifies references discarded by overlay or
// namespace precedence never reach the resolver.
func TestBuildSkipsShadowedReferences(t *testing.T) {
	t.Parallel()

	t.Run("overlay winner", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeYAML(t, dir, "app.yaml", "password: secret://stale\n")
		writeYAML(t, dir, "app.production.yaml", "password: replacement\n")

		res, err := Build(Params{
			Includes:      []string{filepath.Join(dir, "app")},
			Environments:  []string{"production"},
			ValueResolver: fakeResolver{fail: "secret://stale"},
		})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if value, _ := res.Get("PASSWORD"); value != "replacement" {
			t.Errorf("PASSWORD = %q, want replacement", value)
		}
	})

	t.Run("namespace winner", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeYAML(t, dir, "first.yaml", "password: secret://stale\n")
		writeYAML(t, dir, "second.yaml", "password: replacement\n")

		res, err := Build(Params{
			Includes: []string{
				filepath.Join(dir, "first"),
				filepath.Join(dir, "second"),
			},
			Environments:  []string{"production"},
			ValueResolver: fakeResolver{fail: "secret://stale"},
		})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if value, _ := res.Get("PASSWORD"); value != "replacement" {
			t.Errorf("PASSWORD = %q, want replacement", value)
		}
	})
}

// TestBuildResolvesListReferences verifies references inside a list are
// dereferenced per item after winner selection and before the list is joined.
func TestBuildResolvesListReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "tokens:\n  - secret://a\n  - secret://b\n")

	res, err := Build(Params{
		Includes:     []string{filepath.Join(dir, "app")},
		Environments: []string{"development"},
		ValueResolver: fakeResolver{values: map[string]string{
			"secret://a": "tok-a",
			"secret://b": "tok-b",
		}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if v, _ := res.Get("TOKENS"); v != "tok-a,tok-b" {
		t.Errorf("TOKENS = %q, want tok-a,tok-b (list items resolved)", v)
	}
}

// TestBuildRedactsResolvedListItemErrors verifies rendering errors identify a
// list item's location without exposing its resolved plaintext.
func TestBuildRedactsResolvedListItemErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "tokens:\n  - secret://sensitive\n")

	res, err := Build(Params{
		Includes:     []string{filepath.Join(dir, "app")},
		Environments: []string{"development"},
		ValueResolver: fakeResolver{values: map[string]string{
			"secret://sensitive": "plaintext,secret",
		}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	err = res.Verify()
	if err == nil {
		t.Fatal("expected delimiter error")
	}
	if strings.Contains(err.Error(), "plaintext") {
		t.Errorf("error exposes resolved value: %v", err)
	}
	if !strings.Contains(err.Error(), `list item 1 at "tokens"`) {
		t.Errorf("error does not identify the list item: %v", err)
	}
}
