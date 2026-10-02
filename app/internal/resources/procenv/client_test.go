package procenv_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/resources/procenv"
)

func newClient(t *testing.T) *procenv.Client {
	t.Helper()
	client, err := procenv.New(procenv.Params{})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return client
}

func TestLookupEnv(t *testing.T) {
	t.Setenv("PROCENV_PRESENT", "value")
	t.Setenv("PROCENV_EMPTY", "")

	client := newClient(t)

	if got, ok := client.LookupEnv("PROCENV_PRESENT"); !ok || got != "value" {
		t.Errorf("LookupEnv(present) = %q, %v; want value, true", got, ok)
	}
	if got, ok := client.LookupEnv("PROCENV_EMPTY"); !ok || got != "" {
		t.Errorf("LookupEnv(empty) = %q, %v; want empty, true", got, ok)
	}
	if _, ok := client.LookupEnv("PROCENV_ABSENT_VARIABLE"); ok {
		t.Error("LookupEnv(absent) reported present")
	}
}

func TestEnvironSnapshot(t *testing.T) {
	t.Setenv("PROCENV_SNAPSHOT", "a=b")

	snapshot := newClient(t).Environ()

	if got := snapshot["PROCENV_SNAPSHOT"]; got != "a=b" {
		t.Errorf("Environ()[PROCENV_SNAPSHOT] = %q, want a=b", got)
	}

	snapshot["PROCENV_SNAPSHOT"] = "mutated"
	if got := newClient(t).Environ()["PROCENV_SNAPSHOT"]; got != "a=b" {
		t.Errorf("mutating a snapshot leaked into the process environment: %q", got)
	}
}
