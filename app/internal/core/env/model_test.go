package env

import "testing"

// TestEnvironmentCopies verifies the zero Environment is empty and accessors
// do not expose internal state.
func TestEnvironmentCopies(t *testing.T) {
	t.Parallel()

	var zero Environment
	if len(zero.All()) != 0 {
		t.Error("zero Environment is not empty")
	}
	if _, ok := zero.Get("X"); ok {
		t.Error("zero Environment has X")
	}

	e := Environment{
		values:  map[string]string{"A": "1"},
		origins: map[string]Origin{"A": {Shadowed: []Source{{Key: "a"}}}},
	}
	e.All()["A"] = "mutated"
	origin, _ := e.Origin("A")
	origin.Shadowed[0].Key = "mutated"
	if v, _ := e.Get("A"); v != "1" {
		t.Errorf("A = %q after mutating All", v)
	}
	if o, _ := e.Origin("A"); o.Shadowed[0].Key != "a" {
		t.Errorf("origin mutated through copy: %+v", o)
	}
}
