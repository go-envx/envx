package secrets

import "testing"

// -------------------------------------------------------------------------------------

// TestLookup verifies lookup reports both the value and presence of an entry,
// and reports absence for a missing key or group, against an in-memory store.
func TestLookup(t *testing.T) {
	t.Parallel()

	s := &store{secrets: map[reference]string{
		{group: "production", key: "postgres_password"}: "prod-pw",
	}}

	ref := reference{group: "production", key: "postgres_password"}
	if v, ok := s.lookup(ref); !ok || v != "prod-pw" {
		t.Errorf("lookup = %q, %v; want prod-pw, true", v, ok)
	}
	if _, ok := s.lookup(reference{group: "production", key: "missing"}); ok {
		t.Error("expected missing key to be absent")
	}
	if _, ok := s.lookup(reference{group: "ghost", key: "x"}); ok {
		t.Error("expected missing group to be absent")
	}
}
