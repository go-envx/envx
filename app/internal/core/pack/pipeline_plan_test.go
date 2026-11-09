package pack

import (
	"maps"
	"strings"
	"testing"
)

// TestAssignProjectNames verifies stems come from each include's final
// segment and collide-free stems are assigned in include order.
func TestAssignProjectNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		includes []includeFiles
		want     map[string]string
	}{
		{
			name: "final segments",
			includes: []includeFiles{
				{rel: "env/postgres", base: "x"},
				{rel: "apps/api/env/api", base: "x"},
				{rel: "../outside", base: "x"},
			},
			want: map[string]string{
				"env/postgres":     "postgres",
				"apps/api/env/api": "api",
				"../outside":       "outside",
			},
		},
		{
			name: "shared final segment",
			includes: []includeFiles{
				{rel: "env/app", base: "x"},
				{rel: "svc/app", base: "x"},
			},
			want: map[string]string{"env/app": "app", "svc/app": "app-2"},
		},
		{
			name: "shared stem with disjoint files",
			includes: []includeFiles{
				{rel: "env/app", base: "x"},
				{rel: "svc/app", overlays: []overlayFile{{env: "development", src: "y"}}},
			},
			want: map[string]string{"env/app": "app", "svc/app": "app-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := assignProjectNames(tt.includes)
			if err != nil {
				t.Fatalf("assignProjectNames: %v", err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("names = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAssignProjectDirs verifies directories use sanitized project names,
// avoid reserved root names, and disambiguate sanitized collisions in order.
func TestAssignProjectDirs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		projects []string
		reserved []string
		want     map[string]string
	}{
		{
			name:     "safe names",
			projects: []string{"api", "web"},
			want:     map[string]string{"api": "api", "web": "web"},
		},
		{
			name:     "reserved names",
			projects: []string{"envx", "secrets"},
			reserved: []string{"envx.yaml", "envx", "secrets.yaml", "secrets"},
			want:     map[string]string{"envx": "envx-2", "secrets": "secrets-2"},
		},
		{
			name:     "sanitized collision",
			projects: []string{"a/b", "a:b"},
			want:     map[string]string{"a/b": "a_b", "a:b": "a_b-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			projects := make([]project, len(tt.projects))
			for i, name := range tt.projects {
				projects[i] = project{name: name}
			}
			got, err := assignProjectDirs(projects, tt.reserved)
			if err != nil {
				t.Fatalf("assignProjectDirs: %v", err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("dirs = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPlanRejectsOverlongNames verifies names beyond the filename limit fail
// clearly rather than produce invalid paths.
func TestPlanRejectsOverlongNames(t *testing.T) {
	t.Parallel()
	overlays := []overlayFile{{env: strings.Repeat("e", 260), src: "x"}}
	_, err := assignProjectNames(
		[]includeFiles{{rel: "env/app", base: "x", overlays: overlays}},
	)
	if err == nil || !strings.Contains(err.Error(), "filename limit") {
		t.Errorf("assignProjectNames err = %v, want a filename-limit error", err)
	}
	_, err = assignProjectDirs([]project{{name: strings.Repeat("p", 300)}}, nil)
	if err == nil || !strings.Contains(err.Error(), "filename limit") {
		t.Errorf("assignProjectDirs err = %v, want a filename-limit error", err)
	}
}

// FuzzSanitizeDirName verifies every project name becomes one safe, non-empty
// path component that cannot escape the bundle root.
func FuzzSanitizeDirName(f *testing.F) {
	for _, seed := range []string{"api", "a/b", "../escape", "..", ".", "", "a__b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, name string) {
		got := sanitizeDirName(name)
		if got == "" || strings.Contains(got, "__") ||
			strings.ContainsFunc(got, func(r rune) bool {
				return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
					(r < '0' || r > '9') && r != '-' && r != '_'
			}) {
			t.Errorf("sanitizeDirName(%q) = %q, want one safe component", name, got)
		}
	})
}

// TestSanitizeDirName verifies representative mappings.
func TestSanitizeDirName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"api":       "api",
		"api-svc_1": "api-svc_1",
		"../escape": "_escape",
		"":          "project",
		"a...b":     "a_b",
		"a_-_b":     "a_-_b",
	}
	for in, want := range cases {
		if got := sanitizeDirName(in); got != want {
			t.Errorf("sanitizeDirName(%q) = %q, want %q", in, got, want)
		}
	}
}
