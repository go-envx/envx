package pack

import (
	"strings"
	"testing"
)

// TestAssignFlatNamesBasenames verifies each namespace flattens to the final
// segment of its include, regardless of how deep its directory is.
func TestAssignFlatNamesBasenames(t *testing.T) {
	t.Parallel()
	includes := []includeFiles{
		{rel: "env/postgres", base: "x"},
		{rel: "apps/api/env/api", base: "x"},
		{rel: "../outside", base: "x"},
	}
	names, err := assignFlatNames(includes, nil)
	if err != nil {
		t.Fatalf("assignFlatNames: %v", err)
	}
	want := map[string]string{
		"env/postgres":     "postgres",
		"apps/api/env/api": "api",
		"../outside":       "outside",
	}
	for rel, expected := range want {
		if names[rel] != expected {
			t.Errorf("%s = %q, want %q", rel, names[rel], expected)
		}
	}
}

// TestAssignFlatNamesDisambiguatesCollisions verifies two namespaces that share a
// basename in different directories get distinct names, assigned deterministically
// in sorted include order.
func TestAssignFlatNamesDisambiguatesCollisions(t *testing.T) {
	t.Parallel()
	includes := []includeFiles{
		{rel: "svc/app", base: "x"},
		{rel: "env/app", base: "x"},
	}
	names, err := assignFlatNames(includes, nil)
	if err != nil {
		t.Fatalf("assignFlatNames: %v", err)
	}
	// "env/app" sorts before "svc/app", so it keeps the plain "app".
	if names["env/app"] != "app" {
		t.Errorf("env/app = %q, want app", names["env/app"])
	}
	if names["svc/app"] != "app-2" {
		t.Errorf("svc/app = %q, want app-2", names["svc/app"])
	}
}

// TestAssignFlatNamesAvoidsReserved verifies a namespace never flattens onto a
// reserved name (the manifest or secrets store stem).
func TestAssignFlatNamesAvoidsReserved(t *testing.T) {
	t.Parallel()
	includes := []includeFiles{{rel: "env/secrets", base: "x"}}
	names, err := assignFlatNames(includes, []string{"envx", "secrets"})
	if err != nil {
		t.Fatalf("assignFlatNames: %v", err)
	}
	if names["env/secrets"] == "secrets" {
		t.Error("namespace flattened onto the reserved secrets name")
	}
	if names["env/secrets"] != "secrets-2" {
		t.Errorf("env/secrets = %q, want secrets-2", names["env/secrets"])
	}
}

// TestAssignFlatNamesTruncatesToFit verifies a stem is truncated so the longest
// filename it produces stays within the filename limit, and collisions after
// truncation are still disambiguated.
func TestAssignFlatNamesTruncatesToFit(t *testing.T) {
	t.Parallel()
	longEnv := strings.Repeat("e", 200)
	longBase := strings.Repeat("a", 100)
	overlays := []overlayFile{{env: longEnv, src: "x"}}
	// Both share a parent and truncate to the same stem, so the guard must both
	// truncate to fit and disambiguate the collision it creates.
	includes := []includeFiles{
		{rel: "d/" + longBase, base: "x", overlays: overlays},
		{rel: "d/" + longBase + "x", base: "x", overlays: overlays},
	}
	names, err := assignFlatNames(includes, nil)
	if err != nil {
		t.Fatalf("assignFlatNames: %v", err)
	}

	suffix := len("." + longEnv + ".yaml")
	seen := make(map[string]bool)
	disambiguated := false
	for rel, stem := range names {
		if total := len(stem) + suffix; total > maxFilenameLen {
			t.Errorf("%s: filename %d bytes exceeds limit %d", rel, total, maxFilenameLen)
		}
		if seen[stem] {
			t.Errorf("%s: duplicate stem %q after truncation", rel, stem)
		}
		seen[stem] = true
		if strings.HasSuffix(stem, "-2") {
			disambiguated = true
		}
	}
	if !disambiguated {
		t.Error("expected a truncation collision to be disambiguated with -2")
	}
}

// TestAssignFlatNamesRejectsUnflattenable verifies an environment name so long
// that no stem can fit is a clear error rather than an invalid filename.
func TestAssignFlatNamesRejectsUnflattenable(t *testing.T) {
	t.Parallel()
	overlays := []overlayFile{{env: strings.Repeat("e", 260), src: "x"}}
	includes := []includeFiles{{rel: "env/app", base: "x", overlays: overlays}}
	if _, err := assignFlatNames(includes, nil); err == nil ||
		!strings.Contains(err.Error(), "filename limit") {
		t.Fatalf("err = %v, want a filename-limit error", err)
	}
}

// TestClampBytes verifies truncation respects the byte budget and never splits a
// multi-byte rune.
func TestClampBytes(t *testing.T) {
	t.Parallel()
	if got := clampBytes("hello", 10); got != "hello" {
		t.Errorf("clampBytes short = %q, want hello", got)
	}
	if got := clampBytes("hello", 3); got != "hel" {
		t.Errorf("clampBytes = %q, want hel", got)
	}
	if got := clampBytes("héllo", 2); got != "h" {
		// "é" is two bytes; it cannot fit in the one remaining byte, so it is dropped.
		t.Errorf("clampBytes multibyte = %q, want h", got)
	}
	if got := clampBytes("x", 0); got != "" {
		t.Errorf("clampBytes zero = %q, want empty", got)
	}
}
