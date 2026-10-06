package hostenv_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/resources/hostenv"
)

func newClient(t *testing.T) *hostenv.Client {
	t.Helper()
	return hostenv.New()
}

func TestGet(t *testing.T) {
	t.Setenv("HOSTENV_PRESENT", "value")
	t.Setenv("HOSTENV_EMPTY", "")

	client := newClient(t)

	if got, ok := client.Get("HOSTENV_PRESENT"); !ok || got != "value" {
		t.Errorf("Get(present) = %q, %v; want value, true", got, ok)
	}
	if got, ok := client.Get("HOSTENV_EMPTY"); !ok || got != "" {
		t.Errorf("Get(empty) = %q, %v; want empty, true", got, ok)
	}
	if _, ok := client.Get("HOSTENV_ABSENT_VARIABLE"); ok {
		t.Error("Get(absent) reported present")
	}
}

func TestSnapshotIsolation(t *testing.T) {
	t.Setenv("HOSTENV_SNAPSHOT", "a=b")

	client := newClient(t)
	all := client.All()

	if got := all["HOSTENV_SNAPSHOT"]; got != "a=b" {
		t.Errorf("All()[HOSTENV_SNAPSHOT] = %q, want a=b", got)
	}

	// Mutating the returned map must not alter subsequent All() calls or Get()
	all["HOSTENV_SNAPSHOT"] = "mutated"
	if got := client.All()["HOSTENV_SNAPSHOT"]; got != "a=b" {
		t.Errorf("mutating All() leaked into internal snapshot: %q", got)
	}
	if got, ok := client.Get("HOSTENV_SNAPSHOT"); !ok || got != "a=b" {
		t.Errorf("Get() after mutating All() = %q, want a=b", got)
	}

	// Subsequent process environment mutations after client creation must not
	// affect the frozen snapshot.
	t.Setenv("HOSTENV_SNAPSHOT", "changed_after_new")
	if got, _ := client.Get("HOSTENV_SNAPSHOT"); got != "a=b" {
		t.Errorf("snapshot was affected by post-New setenv: %q, want a=b", got)
	}
}
