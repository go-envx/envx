package diff

import (
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// -------------------------------------------------------------------------------------

// TestBuildEnvDoesNotMutateConfig verifies buildEnv resolves a side from a
// copy of the shared config, leaving the caller's Settings untouched so both diff
// sides resolve from identical settings save for the overridden environment.
func TestBuildEnvDoesNotMutateConfig(t *testing.T) {
	t.Parallel()

	ec := &envmerge.Params{
		Environments: []string{"development", "production"},
		Settings:     envmerge.Settings{Env: "development", Prefix: "P_"},
	}

	if _, err := buildEnv(*ec, "production"); err != nil {
		t.Fatalf("buildEnv: %v", err)
	}

	if ec.Settings.Env != "development" {
		t.Errorf(
			"shared config Env mutated: got %q, want %q",
			ec.Settings.Env, "development",
		)
	}
}

// -------------------------------------------------------------------------------------

// TestUnionKeys verifies the sorted union of two key sets.
func TestUnionKeys(t *testing.T) {
	t.Parallel()

	a := map[string]string{"B": "1", "A": "2"}
	b := map[string]string{"C": "3", "A": "9"}

	got := unionKeys(a, b)
	want := []string{"A", "B", "C"}
	if len(got) != len(want) {
		t.Fatalf("unionKeys len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unionKeys[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// -------------------------------------------------------------------------------------

// TestRunActionChangedValue verifies the pure core reports a key whose value
// differs between the two environments as a change carrying both sides' values.
func TestRunActionChangedValue(t *testing.T) {
	t.Parallel()

	a, b := diffSides(t, "development", "production")
	res := runAction(a, b)

	got, ok := findChange(res.Changed, "HOST")
	if !ok {
		t.Fatalf("HOST missing from changed set: %+v", res.Changed)
	}
	if got.EnvA != "dev-db.local" || got.EnvB != "prod-db.internal" {
		t.Errorf(
			"HOST change = %q -> %q, want dev-db.local -> prod-db.internal",
			got.EnvA, got.EnvB,
		)
	}
}

// -------------------------------------------------------------------------------------

// TestRunActionIdenticalEnvs verifies diffing an environment against itself
// yields no differences.
func TestRunActionIdenticalEnvs(t *testing.T) {
	t.Parallel()

	a, _ := diffSides(t, "development", "development")
	res := runAction(a, a)

	if len(res.Added) != 0 || len(res.Removed) != 0 || len(res.Changed) != 0 {
		t.Errorf("expected empty diff, got %+v", res)
	}
}

// -------------------------------------------------------------------------------------

// diffSides resolves the api-core project from the shared "basic" fixture and
// builds it under two environments, returning both merged results.
func diffSides(t *testing.T, envA, envB string) (a, b *envmerge.Result) {
	t.Helper()
	path := fixtures.Manifest("basic")
	ec, err := config.Resolve(&config.Input{ConfigPath: &path}, "api-core")
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	a, err = buildEnv(*ec, envA)
	if err != nil {
		t.Fatalf("buildEnv %s: %v", envA, err)
	}
	b, err = buildEnv(*ec, envB)
	if err != nil {
		t.Fatalf("buildEnv %s: %v", envB, err)
	}
	return a, b
}

// -------------------------------------------------------------------------------------

// findChange returns the change with the given key from a slice, and whether it
// was present.
func findChange(changes []actionResultChange, key string) (actionResultChange, bool) {
	for _, c := range changes {
		if c.Key == key {
			return c, true
		}
	}
	return actionResultChange{}, false
}
