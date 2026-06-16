package manifest

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/file"
	"github.com/go-envx/envx/apps/envx/internal/str"
)

// -------------------------------------------------------------------------------------
// TestLoadValidManifest verifies that a well-formed manifest loads with the
// correct directory, settings, environments, and projects.
func TestLoadValidManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `
		environments: [development, staging, production]
		settings:
		  overload: true
		  strict: true
		projects:
		  api-core:
		    path: apps/api-core/env
		    includes:
		      - env/postgres
		      - env/gateway
		  web:
		    path: apps/web/env
		    includes:
		      - env/gateway
	`
	file.Write(t, dir, "envx.yaml", content)

	m, err := Load(filepath.Join(dir, "envx.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", m.Dir(), dir)
	}
	if m.Settings.Overload == nil || !*m.Settings.Overload {
		t.Error("expected Settings.Overload to be true")
	}
	if m.Settings.Strict == nil || !*m.Settings.Strict {
		t.Error("expected Settings.Strict to be true")
	}
	if len(m.Environments) != 3 {
		t.Errorf("got %d environments, want 3", len(m.Environments))
	}
	if len(m.Projects) != 2 {
		t.Errorf("got %d projects, want 2", len(m.Projects))
	}
}

// -------------------------------------------------------------------------------------
// TestLoadFileNotFound verifies that loading a nonexistent file returns an error.
func TestLoadFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := Load("/nonexistent/path/envx.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// -------------------------------------------------------------------------------------
// TestLoadInvalidYAML verifies that malformed YAML returns a parse error.
func TestLoadInvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "envx.yaml", ":\n  invalid:\nyaml: [")

	_, err := Load(filepath.Join(dir, "envx.yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// -------------------------------------------------------------------------------------
// TestValidationErrors verifies that structural constraint violations are
// caught: missing environments, missing projects, empty paths, duplicate
// paths, and empty includes.
func TestValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "no environments",
			content: `
				projects:
				  app:
				    path: x
			`,
			wantErr: "environments list must not be empty",
		},
		{
			name:    "no projects",
			content: "environments: [dev]\n",
			wantErr: "at least one project must be defined",
		},
		{
			name: "project missing path",
			content: `
				environments: [dev]
				projects:
				  app:
				    includes: []
			`,
			wantErr: "has no path",
		},
		{
			name: "duplicate paths",
			content: `
				environments: [dev]
				projects:
				  a:
				    path: same
				  b:
				    path: same
			`,
			wantErr: "share the same path",
		},
		{
			name: "empty include",
			content: `
				environments: [dev]
				projects:
				  app:
				    path: x
				    includes:
				      - ""
			`,
			wantErr: "empty include",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			file.Write(t, dir, "envx.yaml", tt.content)

			_, err := Load(filepath.Join(dir, "envx.yaml"))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !str.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestLoadParsesSettings verifies that global and project-level settings
// (prefix, suffix, namespace_prefix, overload) are correctly parsed.
func TestLoadParsesSettings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `
		environments: [dev]
		settings:
		  prefix: MY_
		  suffix: _ENV
		  namespace_prefix: false
		projects:
		  app:
		    path: apps/app/env
		    settings:
		      overload: true
	`
	file.Write(t, dir, "envx.yaml", content)

	m, err := Load(filepath.Join(dir, "envx.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Settings.Prefix != "MY_" {
		t.Errorf("Prefix = %q, want %q", m.Settings.Prefix, "MY_")
	}
	if m.Settings.Suffix != "_ENV" {
		t.Errorf("Suffix = %q, want %q", m.Settings.Suffix, "_ENV")
	}
	if m.Settings.NamespacePrefix == nil || *m.Settings.NamespacePrefix {
		t.Error("expected NamespacePrefix to be false")
	}

	proj := m.Projects["app"]
	if proj.Settings.Overload == nil || !*proj.Settings.Overload {
		t.Error("expected project Overload to be true")
	}
}
