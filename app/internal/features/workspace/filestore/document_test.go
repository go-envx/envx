package filestore

import (
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// TestParseDocumentDetectsIndent verifies parseDocument reports the source
// document's block indentation and defaults to two spaces when none is detectable.
func TestParseDocumentDetectsIndent(t *testing.T) {
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
			ws, err := parseDocument([]byte(tc.body), "/path/to/envx.yaml")
			if err != nil {
				t.Fatalf("parseDocument: %v", err)
			}
			if ws.Indent != tc.want {
				t.Errorf("Indent = %d, want %d", ws.Indent, tc.want)
			}
		})
	}
}

// TestParseDocumentInvalid verifies structural validation rejects malformed manifests.
func TestParseDocumentInvalid(t *testing.T) {
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
			if _, err := parseDocument([]byte(body), "/path/to/envx.yaml"); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// TestParseDocumentRejectsUnknownFields verifies strict decoding fails a manifest that
// carries an unknown key rather than silently dropping it.
func TestParseDocumentRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body      string
		wantLabel string
	}{
		"unknown top-level key": {
			body: "environments: [development]\n" +
				"projects:\n  api:\n    includes: [env/x]\n" +
				"bogus: true\n",
			wantLabel: "manifest key",
		},
		"unknown settings key": {
			body: "environments: [development]\n" +
				"settings:\n  not_a_setting: true\n" +
				"projects:\n  api:\n    includes: [env/x]\n",
			wantLabel: "setting",
		},
		"unknown secrets key": {
			body: "environments: [development]\n" +
				"secrets:\n  not_a_secret: true\n" +
				"projects:\n  api:\n    includes: [env/x]\n",
			wantLabel: "secrets setting",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := parseDocument([]byte(tc.body), "/path/to/envx.yaml")
			if err == nil {
				t.Fatal("expected an error for an unknown manifest key")
			}
			got := err.Error()
			if !strings.Contains(got, "unknown "+tc.wantLabel) {
				t.Errorf("error %q should label the block as %q", got, tc.wantLabel)
			}
			if strings.Contains(got, "schema.") || strings.Contains(got, "yaml:") {
				t.Errorf("error %q should not leak internal decoder/type details", got)
			}
			if !strings.Contains(got, workspace.SchemaDocsURL) {
				t.Errorf("error %q should point at the schema docs", got)
			}
		})
	}
}

// TestParseDocumentSuggestsNearestKey verifies a rejected key close to a valid
// one offers a "did you mean" correction.
func TestParseDocumentSuggestsNearestKey(t *testing.T) {
	t.Parallel()

	body := "environments: [development]\n" +
		"settings:\n  require_overlays: false\n" +
		"projects:\n  api:\n    includes: [env/x]\n"

	_, err := parseDocument([]byte(body), "/path/to/envx.yaml")
	if err == nil {
		t.Fatal("expected an error for a renamed settings key")
	}
	if !strings.Contains(err.Error(), `Did you mean "require-overlays"?`) {
		t.Errorf("error %q should suggest the nearest valid key", err)
	}
}

// TestParseDocumentKebabSettingsKeys verifies the v2 kebab-case settings keys
// decode into their fields, guarding the yaml-tag rename from snake_case.
func TestParseDocumentKebabSettingsKeys(t *testing.T) {
	t.Parallel()

	body := "environments: [development]\n" +
		"settings:\n" +
		"  reference-pattern: '<<(.+)>>'\n" +
		"  require-overlays: true\n" +
		"projects:\n  api:\n    includes: [env/x]\n"

	ws, err := parseDocument([]byte(body), "/path/to/envx.yaml")
	if err != nil {
		t.Fatalf("parseDocument: %v", err)
	}
	s := ws.Settings
	if s.ReferencePattern == nil || *s.ReferencePattern != "<<(.+)>>" {
		t.Errorf("ReferencePattern = %v, want <<(.+)>>", s.ReferencePattern)
	}
	if s.RequireOverlays == nil || !*s.RequireOverlays {
		t.Errorf("RequireOverlays = %v, want true", s.RequireOverlays)
	}
}
