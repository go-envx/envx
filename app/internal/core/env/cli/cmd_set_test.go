package cli

import (
	"slices"
	"strings"
	"testing"
)

// TestSetCommandWritesOverlay verifies the arguments and --env reach the
// overlay writer and the written overlay is confirmed.
func TestSetCommandWritesOverlay(t *testing.T) {
	t.Parallel()

	factory := newFakeFactory()
	stdout, _, err := execute(
		t, NewSetCommand(factory),
		"env/postgres", "host", "prod-db.local", "--env", "production",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}

	want := []overlayWrite{{
		include:     appInclude,
		environment: "production",
		key:         "host",
		text:        "prod-db.local",
	}}
	if !slices.Equal(factory.overlays, want) {
		t.Errorf("overlays = %+v, want %+v", factory.overlays, want)
	}
	if !strings.Contains(stdout, `Set "host" in:`) ||
		!strings.Contains(stdout, appInclude+".production.yaml") {
		t.Errorf("stdout = %q, want the set confirmation", stdout)
	}
}
