package diff

import (
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// TestBuildEngineDoesNotMutateConfig verifies buildEngine resolves a side from a
// copy of the shared config, leaving the caller's Settings untouched so both diff
// sides resolve from identical settings save for the overridden environment.
func TestBuildEngineDoesNotMutateConfig(t *testing.T) {
	t.Parallel()

	ec := &engine.Config{
		Environments: []string{"development", "production"},
		Settings:     engine.Settings{Env: "development", Prefix: "P_"},
	}

	if _, err := buildEngine(ec, "production"); err != nil {
		t.Fatalf("buildEngine: %v", err)
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
// TestMaskResult verifies values are redacted unless reveal is set, and that
// empty values stay empty.
func TestMaskResult(t *testing.T) {
	t.Parallel()

	in := actionResult{
		Added:   []change{{Key: "A", EnvB: "secret"}},
		Removed: []change{{Key: "B", EnvA: "gone"}},
		Changed: []change{{Key: "C", EnvA: "old", EnvB: "new"}},
	}

	masked := maskResult(in, false)
	if masked.Added[0].EnvB != redacted {
		t.Errorf("added env-b = %q, want redacted", masked.Added[0].EnvB)
	}
	if masked.Removed[0].EnvA != redacted {
		t.Errorf("removed env-a = %q, want redacted", masked.Removed[0].EnvA)
	}
	if masked.Changed[0].EnvA != redacted || masked.Changed[0].EnvB != redacted {
		t.Errorf("changed not fully redacted: %+v", masked.Changed[0])
	}

	revealed := maskResult(in, true)
	if revealed.Added[0].EnvB != "secret" {
		t.Errorf("reveal should keep value, got %q", revealed.Added[0].EnvB)
	}
}
