package env

import (
	"errors"
	"slices"
	"testing"
)

// webInclude is a second fixture namespace.
const webInclude = "/ws/env/web"

// mergeFixture merges namespaces for environment with optional settings.
func mergeFixture(
	t *testing.T, namespaces *fakeNamespaces, environment string,
	includes []string, set resolvedSettings,
) (*mergedEnv, error) {
	t.Helper()
	s := newService(t, testWorkspace(), Dependencies{Namespaces: namespaces})
	loaded, err := s.loadNamespaces(includes)
	if err != nil {
		return nil, err
	}
	return s.merge(loaded, operation{environment: environment, settings: set})
}

// TestMergeOverlay verifies overlays win, nested and flat spellings override
// across files, and partial nested overrides keep base siblings.
func TestMergeOverlay(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().
		base(t, appInclude, `
			host: localhost
			port: 5432
			log:
			  level: info
			  format: json
		`).
		overlay(t, appInclude, "production", `
			host: prod-db
			log_level: warn
		`)
	merged, err := mergeFixture(
		t, namespaces, "production", []string{appInclude}, resolvedSettings{},
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	want := map[string]string{
		"HOST": "prod-db", "PORT": "5432", "LOG_LEVEL": "warn", "LOG_FORMAT": "json",
	}
	for key, text := range want {
		if got := literalOf(merged.values[key], ","); got != text {
			t.Errorf("%s = %q, want %q", key, got, text)
		}
	}

	level := merged.origins["LOG_LEVEL"]
	wantLevel := Origin{
		Winner:   Source{Path: appInclude + ".production.yaml", Key: "log_level"},
		Shadowed: []Source{{Path: appInclude + ".yaml", Key: "log.level"}},
	}
	if !originEqual(level, wantLevel) {
		t.Errorf("LOG_LEVEL origin = %+v, want %+v", level, wantLevel)
	}
	if len(merged.origins["PORT"].Shadowed) != 0 {
		t.Errorf("PORT shadowed = %v, want none", merged.origins["PORT"].Shadowed)
	}
}

// TestMergeAcrossNamespaces verifies later namespaces win and earlier
// winners are demoted oldest first.
func TestMergeAcrossNamespaces(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().
		base(t, appInclude, "host: app").
		overlay(t, appInclude, "development", "host: app-dev").
		base(t, webInclude, "host: web")
	merged, err := mergeFixture(
		t, namespaces, "development", []string{appInclude, webInclude}, resolvedSettings{},
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	want := Origin{
		Winner: Source{Path: webInclude + ".yaml", Key: "host"},
		Shadowed: []Source{
			{Path: appInclude + ".yaml", Key: "host"},
			{Path: appInclude + ".development.yaml", Key: "host"},
		},
	}
	if got := merged.origins["HOST"]; !originEqual(got, want) {
		t.Errorf("HOST origin = %+v, want %+v", got, want)
	}
}

// TestMergeDeclaredInBase verifies overlay-only keys are not declared in a
// base while keys any base declares are.
func TestMergeDeclaredInBase(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().
		base(t, appInclude, "host: a").
		overlay(t, appInclude, "development", "host: b\nextra: c")
	merged, err := mergeFixture(
		t, namespaces, "development", []string{appInclude},
		resolvedSettings{prefix: "app"},
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !merged.declaredInBase["APP_HOST"] || merged.declaredInBase["APP_EXTRA"] {
		t.Errorf("declaredInBase = %v, want only APP_HOST", merged.declaredInBase)
	}
}

// TestMergeErrors verifies structural failures are fatal.
func TestMergeErrors(t *testing.T) {
	t.Parallel()

	missingOverlay := newFakeNamespaces().base(t, appInclude, "host: a")
	_, err := mergeFixture(
		t, missingOverlay, "production", []string{appInclude},
		resolvedSettings{requireOverlays: true},
	)
	if !errors.Is(err, ErrOverlayNotFound) {
		t.Errorf("required overlay error = %v, want ErrOverlayNotFound", err)
	}
	if _, err := mergeFixture(
		t, missingOverlay, "production", []string{appInclude}, resolvedSettings{},
	); err != nil {
		t.Errorf("optional overlay error = %v", err)
	}

	collision := newFakeNamespaces().base(t, appInclude, "log:\n  level: a\nlog_level: b")
	_, err = mergeFixture(
		t, collision, "development", []string{appInclude}, resolvedSettings{},
	)
	if !errors.Is(err, ErrFlattenCollision) {
		t.Errorf("collision error = %v, want ErrFlattenCollision", err)
	}

	_, err = mergeFixture(
		t, newFakeNamespaces(), "development", []string{appInclude}, resolvedSettings{},
	)
	if err == nil {
		t.Error("missing base merged")
	}
}

// TestMergeAffixes verifies prefix and suffix are uppercased and joined once.
func TestMergeAffixes(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().base(t, appInclude, "host: a")
	merged, err := mergeFixture(
		t, namespaces, "development", []string{appInclude},
		resolvedSettings{prefix: "app_", suffix: "_v2"},
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, ok := merged.values["APP_HOST_V2"]; !ok {
		t.Errorf("values = %v, want APP_HOST_V2", merged.values)
	}
	if _, ok := merged.origins["APP_HOST_V2"]; !ok {
		t.Errorf("origins missing APP_HOST_V2")
	}
}

// TestApplyHost verifies host source selection under each overload and union
// combination.
func TestApplyHost(t *testing.T) {
	t.Parallel()

	baseSource := Source{Path: appInclude + ".yaml", Key: "host"}
	hostSource := Source{Key: "HOST", Host: true}
	tests := []struct {
		// name labels the case.
		name string
		// union adds host-only keys.
		union bool
		// overload lets namespace values win.
		overload bool
		// wantHost is the expected HOST literal.
		wantHost string
		// wantOrigin is the expected HOST origin.
		wantOrigin Origin
		// wantOnly reports whether HOST_ONLY is present.
		wantOnly bool
	}{
		{
			name: "host wins", wantHost: "from-host",
			wantOrigin: Origin{Winner: hostSource, Shadowed: []Source{baseSource}},
		},
		{
			name: "overload", overload: true, wantHost: "from-file",
			wantOrigin: Origin{Winner: baseSource, Shadowed: []Source{hostSource}},
		},
		{
			name: "union", union: true, wantHost: "from-host", wantOnly: true,
			wantOrigin: Origin{Winner: hostSource, Shadowed: []Source{baseSource}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			namespaces := newFakeNamespaces().base(t, appInclude, "host: from-file")
			merged, err := mergeFixture(
				t, namespaces, "development", []string{appInclude}, resolvedSettings{},
			)
			if err != nil {
				t.Fatalf("merge: %v", err)
			}
			op := operation{
				host:     hostOf("HOST=from-host", "HOST_ONLY=x")(),
				settings: resolvedSettings{overload: tt.overload},
			}
			merged.applyHost(op, tt.union)

			if got := literalOf(merged.values["HOST"], ","); got != tt.wantHost {
				t.Errorf("HOST = %q, want %q", got, tt.wantHost)
			}
			if got := merged.origins["HOST"]; !originEqual(got, tt.wantOrigin) {
				t.Errorf("HOST origin = %+v, want %+v", got, tt.wantOrigin)
			}
			if _, ok := merged.values["HOST_ONLY"]; ok != tt.wantOnly {
				t.Errorf("HOST_ONLY present = %v, want %v", ok, tt.wantOnly)
			}
			if merged.opaque("HOST") == tt.overload {
				t.Errorf("HOST opaque = %v, want %v", merged.opaque("HOST"), !tt.overload)
			}
		})
	}
}

// TestLoadNamespacesOncePerDiff verifies a diff reads each base once for both
// sides.
func TestLoadNamespacesOncePerDiff(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().base(t, appInclude, "host: a")
	s := newService(t, testWorkspace(), Dependencies{Namespaces: namespaces})
	if _, err := s.Diff(DiffParams{From: "development", To: "production"}); err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if namespaces.baseReads != 1 {
		t.Errorf("base reads = %d, want 1", namespaces.baseReads)
	}
}

// originEqual compares two origins.
func originEqual(a, b Origin) bool {
	return a.Winner == b.Winner && slices.Equal(a.Shadowed, b.Shadowed)
}
