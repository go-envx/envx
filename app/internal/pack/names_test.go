package pack

import (
	"strings"
	"testing"
)

// mustNames runs assignProjectNames and fails the test on error.
func mustNames(t *testing.T, includes []includeFiles) map[string]string {
	t.Helper()
	names, err := assignProjectNames(includes)
	if err != nil {
		t.Fatalf("assignProjectNames: %v", err)
	}
	return names
}

// mustDirs runs assignProjectDirs and fails the test on error.
func mustDirs(t *testing.T, projects []Project, reserved []string) map[string]string {
	t.Helper()
	dirs, err := assignProjectDirs(projects, reserved)
	if err != nil {
		t.Fatalf("assignProjectDirs: %v", err)
	}
	return dirs
}

// TestAssignProjectNamesBasenames verifies each namespace is named after the
// final segment of its include, regardless of how deep its directory is.
func TestAssignProjectNamesBasenames(t *testing.T) {
	t.Parallel()
	includes := []includeFiles{
		{rel: "env/postgres", base: "x"},
		{rel: "apps/api/env/api", base: "x"},
		{rel: "../outside", base: "x"},
	}
	names := mustNames(t, includes)
	want := map[string]string{
		"env/postgres":     "postgres",
		"apps/api/env/api": "api",
		"../outside":       "outside",
	}
	for rel, expected := range want {
		if names[rel] != expected {
			t.Errorf("%s = %q, want %q", rel, names[rel], expected)
		}
	}
}

// TestAssignProjectNamesDisambiguatesCollisions verifies two namespaces in the
// same project that share a final segment get distinct names, assigned in include
// order so the first keeps the plain name.
func TestAssignProjectNamesDisambiguatesCollisions(t *testing.T) {
	t.Parallel()
	includes := []includeFiles{
		{rel: "env/app", base: "x"},
		{rel: "svc/app", base: "x"},
	}
	names := mustNames(t, includes)
	if names["env/app"] != "app" {
		t.Errorf("env/app = %q, want app", names["env/app"])
	}
	if names["svc/app"] != "app-2" {
		t.Errorf("svc/app = %q, want app-2", names["svc/app"])
	}
}

// TestAssignProjectNamesDisambiguatesDisjointFiles verifies two namespaces that
// share a final segment get distinct stems even when their output files do not
// overlap — a base-only namespace and an overlay-only namespace. Sharing a stem
// would merge them into one include at resolution time.
func TestAssignProjectNamesDisambiguatesDisjointFiles(t *testing.T) {
	t.Parallel()
	// env/app produces app.yaml; svc/app produces only app.development.yaml.
	includes := []includeFiles{
		{rel: "env/app", base: "x"},
		{rel: "svc/app", overlays: []overlayFile{{env: "development", src: "y"}}},
	}
	names := mustNames(t, includes)
	if names["env/app"] == names["svc/app"] {
		t.Errorf(
			"env/app and svc/app share stem %q; disjoint files must still get distinct stems",
			names["env/app"],
		)
	}
	if names["env/app"] != "app" {
		t.Errorf("env/app = %q, want app", names["env/app"])
	}
	if names["svc/app"] != "app-2" {
		t.Errorf("svc/app = %q, want app-2", names["svc/app"])
	}
}

// TestAssignProjectDirsUsesProjectNames verifies each project directory is named
// after its project when the name is already filesystem-safe.
func TestAssignProjectDirsUsesProjectNames(t *testing.T) {
	t.Parallel()
	projects := []Project{{Name: "api"}, {Name: "web"}}
	dirs := mustDirs(t, projects, nil)
	if dirs["api"] != "api" || dirs["web"] != "web" {
		t.Errorf("dirs = %v, want api->api web->web", dirs)
	}
}

// TestAssignProjectDirsAvoidsReserved verifies a project whose sanitized name
// matches a reserved bundle name is disambiguated so no directory shadows a root
// bundle file.
func TestAssignProjectDirsAvoidsReserved(t *testing.T) {
	t.Parallel()
	projects := []Project{{Name: "envx"}, {Name: "secrets"}}
	reserved := []string{"envx.yaml", "envx", "secrets.yaml", "secrets"}
	dirs := mustDirs(t, projects, reserved)
	if dirs["envx"] == "envx" {
		t.Error("project directory shadowed the reserved envx name")
	}
	if dirs["secrets"] == "secrets" {
		t.Error("project directory shadowed the reserved secrets name")
	}
	if dirs["envx"] != "envx-2" {
		t.Errorf("envx = %q, want envx-2", dirs["envx"])
	}
	if dirs["secrets"] != "secrets-2" {
		t.Errorf("secrets = %q, want secrets-2", dirs["secrets"])
	}
}

// TestAssignProjectDirsDisambiguatesSanitizedCollisions verifies two projects
// whose names sanitize to the same string get distinct directories.
func TestAssignProjectDirsDisambiguatesSanitizedCollisions(t *testing.T) {
	t.Parallel()
	projects := []Project{{Name: "a/b"}, {Name: "a:b"}}
	dirs := mustDirs(t, projects, nil)
	if dirs["a/b"] != "a_b" {
		t.Errorf("a/b = %q, want a_b", dirs["a/b"])
	}
	if dirs["a:b"] != "a_b-2" {
		t.Errorf("a:b = %q, want a_b-2", dirs["a:b"])
	}
}

// TestAssignProjectNamesRejectsOverlongFilename verifies an environment name so
// long that the produced filename exceeds the limit is a clear error rather than
// an invalid filename.
func TestAssignProjectNamesRejectsOverlongFilename(t *testing.T) {
	t.Parallel()
	overlays := []overlayFile{{env: strings.Repeat("e", 260), src: "x"}}
	includes := []includeFiles{{rel: "env/app", base: "x", overlays: overlays}}
	if _, err := assignProjectNames(includes); err == nil ||
		!strings.Contains(err.Error(), "filename limit") {
		t.Fatalf("err = %v, want a filename-limit error", err)
	}
}

// TestAssignProjectDirsRejectsOverlongName verifies a project name that exceeds
// the per-component filename limit is a clear error rather than an invalid
// directory.
func TestAssignProjectDirsRejectsOverlongName(t *testing.T) {
	t.Parallel()
	projects := []Project{{Name: strings.Repeat("p", 300)}}
	if _, err := assignProjectDirs(projects, nil); err == nil ||
		!strings.Contains(err.Error(), "filename limit") {
		t.Fatalf("err = %v, want a filename-limit error", err)
	}
}

// TestSanitizeDirName verifies only [A-Za-z0-9_-] survive, every other character
// (including "." and "/") becomes an underscore, consecutive underscores collapse
// to one, and a name that sanitizes to empty falls back to a safe placeholder — so
// a project can never escape the bundle root or produce an invalid directory.
func TestSanitizeDirName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"api":       "api",
		"api-svc_1": "api-svc_1",
		"a/b":       "a_b",
		"../escape": "_escape",
		"..":        "_",
		".":         "_",
		"":          "project",
		"a b":       "a_b",
		"x\\y":      "x_y",
		"a__b":      "a_b",   // pre-existing underscore run collapses
		"a...b":     "a_b",   // dots are no longer allowed
		"a/./b":     "a_b",   // mixed separators collapse to one underscore
		"a_-_b":     "a_-_b", // dashes break an underscore run
	}
	for in, want := range cases {
		if got := sanitizeDirName(in); got != want {
			t.Errorf("sanitizeDirName(%q) = %q, want %q", in, got, want)
		}
	}
}
