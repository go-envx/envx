package engine

import "testing"

// -------------------------------------------------------------------------------------
// TestResultAllIsCopy verifies All returns a defensive copy.
func TestResultAllIsCopy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")
	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}}, "development", mergeOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	all := res.All()
	all["HOST"] = "mutated"
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("internal map was mutated: HOST = %q", v)
	}
}
