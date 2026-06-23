package schema

import "testing"

// -------------------------------------------------------------------------------------

// testManifest builds an in-memory manifest for exercising the pure query
// methods without any file I/O.
func testManifest() *Manifest {
	return &Manifest{
		Environments: []string{"development", "staging", "production"},
		Projects: map[string]Project{
			"api": {Includes: []string{"env/postgres", "apps/api/env/api"}},
			"web": {Includes: []string{"env/web"}},
		},
	}
}

// -------------------------------------------------------------------------------------

// TestLookupProject verifies a known project resolves to its definition and an
// unknown project reports not found.
func TestLookupProject(t *testing.T) {
	t.Parallel()

	m := testManifest()

	proj, ok := m.LookupProject("api")
	if !ok {
		t.Fatal("expected project api to be found")
	}
	if len(proj.Includes) != 2 {
		t.Errorf("Includes = %v, want 2 entries", proj.Includes)
	}

	if _, ok := m.LookupProject("ghost"); ok {
		t.Error("expected unknown project to report not found")
	}
}

// -------------------------------------------------------------------------------------

// TestDefaultEnvironment verifies the default resolves to the first declared
// environment and that an empty list yields "".
func TestDefaultEnvironment(t *testing.T) {
	t.Parallel()

	m := &Manifest{Environments: []string{"staging", "development"}}
	if got := m.DefaultEnvironment(); got != "staging" {
		t.Errorf("DefaultEnvironment = %q, want staging", got)
	}
	if got := (&Manifest{}).DefaultEnvironment(); got != "" {
		t.Errorf("DefaultEnvironment() = %q, want empty", got)
	}
}

// -------------------------------------------------------------------------------------

// TestHasEnvironment verifies declared environments are recognized and an
// undeclared one is rejected.
func TestHasEnvironment(t *testing.T) {
	t.Parallel()

	m := testManifest()

	for _, env := range []string{"development", "staging", "production"} {
		if !m.HasEnvironment(env) {
			t.Errorf("HasEnvironment(%q) = false, want true", env)
		}
	}
	if m.HasEnvironment("qa") {
		t.Error(`HasEnvironment("qa") = true, want false`)
	}
}

// -------------------------------------------------------------------------------------

// TestHasInclude verifies an include declared by any project is found (across
// projects) and an undeclared include is not.
func TestHasInclude(t *testing.T) {
	t.Parallel()

	m := testManifest()

	for _, inc := range []string{"env/postgres", "apps/api/env/api", "env/web"} {
		if !m.HasInclude(inc) {
			t.Errorf("HasInclude(%q) = false, want true", inc)
		}
	}
	if m.HasInclude("env/ghost") {
		t.Error(`HasInclude("env/ghost") = true, want false`)
	}
}
