package diff

import "testing"

// -------------------------------------------------------------------------------------
// TestUnionKeys verifies the sorted union of two key sets.
func TestUnionKeys(t *testing.T) {
	t.Parallel()

	left := map[string]string{"B": "1", "A": "2"}
	right := map[string]string{"C": "3", "A": "9"}

	got := unionKeys(left, right)
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
		Added:   []change{{Key: "A", Right: "secret"}},
		Removed: []change{{Key: "B", Left: "gone"}},
		Changed: []change{{Key: "C", Left: "old", Right: "new"}},
	}

	masked := maskResult(in, false)
	if masked.Added[0].Right != redacted {
		t.Errorf("added right = %q, want redacted", masked.Added[0].Right)
	}
	if masked.Removed[0].Left != redacted {
		t.Errorf("removed left = %q, want redacted", masked.Removed[0].Left)
	}
	if masked.Changed[0].Left != redacted || masked.Changed[0].Right != redacted {
		t.Errorf("changed not fully redacted: %+v", masked.Changed[0])
	}

	revealed := maskResult(in, true)
	if revealed.Added[0].Right != "secret" {
		t.Errorf("reveal should keep value, got %q", revealed.Added[0].Right)
	}
}
