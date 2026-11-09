package env

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

// materializeFixture serves a namespace with optional failing keys.
func materializeFixture(t *testing.T, yaml string, host ...string) *Service {
	t.Helper()
	opener := &fakeOpener{plaintext: map[string]string{"secret://db/password": "s3cr3t"}}
	return newService(t, testWorkspace(), Dependencies{
		Namespaces:   newFakeNamespaces().base(t, appInclude, yaml),
		HostEnv:      hostOf(host...),
		OpenResolver: opener.open,
	})
}

// TestMaterialize verifies the complete environment includes resolved,
// substituted, and host-only values.
func TestMaterialize(t *testing.T) {
	t.Parallel()

	s := materializeFixture(t, `
		password: secret://db/password
		url: "{{HOST}}:{{PASSWORD}}"
		host: file
		opaque: x
	`, "HOST=from-host", "OPAQUE=secret://db/missing", "HOST_ONLY=extra")
	result, err := s.Materialize(MaterializeParams{})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	want := map[string]string{
		"PASSWORD": "s3cr3t", "URL": "from-host:s3cr3t", "HOST": "from-host",
		"OPAQUE": "secret://db/missing", "HOST_ONLY": "extra",
	}
	got := result.Environment.All()
	if len(got) != len(want) {
		t.Errorf("All = %v, want %v", got, want)
	}
	for key, text := range want {
		if got[key] != text {
			t.Errorf("%s = %q, want %q", key, got[key], text)
		}
	}
	if result.Warnings != nil {
		t.Errorf("Warnings = %v, want nil", result.Warnings)
	}
	if origin, ok := result.Environment.Origin("HOST_ONLY"); !ok || !origin.Winner.Host {
		t.Errorf("HOST_ONLY origin = %+v", origin)
	}
}

// TestMaterializeStrict verifies every failure is reported in key order
// without values.
func TestMaterializeStrict(t *testing.T) {
	t.Parallel()

	s := materializeFixture(t, `
		b: secret://db/missing-b
		a: secret://db/missing-a
		password: secret://db/password
	`)
	_, err := s.Materialize(MaterializeParams{})
	if err == nil {
		t.Fatal("Materialize succeeded")
	}
	text := err.Error()
	if strings.Index(text, "resolving A") > strings.Index(text, "resolving B") ||
		strings.Contains(text, "s3cr3t") {
		t.Errorf("error = %q, want A before B and no plaintext", text)
	}

	_, err = materializeFixture(t, `loop: "{{LOOP}}"`).Materialize(MaterializeParams{})
	if !errors.Is(err, ErrCircularReference) {
		t.Errorf("cycle error = %v", err)
	}
}

// TestMaterializeTolerant verifies failures become sorted warnings and keys
// keep host values or are omitted.
func TestMaterializeTolerant(t *testing.T) {
	t.Parallel()

	s := materializeFixture(t, `
		dangling: secret://db/missing
		missing: "{{NOWHERE}}"
		loop: "{{LOOP}}"
		ok: fine
	`, "DANGLING=from-host", "LOOP=from-host")
	overload := workspace.Settings{Overload: ptr(true)}
	result, err := s.Materialize(MaterializeParams{
		Query: Query{Options: overload}, IgnoreErrors: true,
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	all := result.Environment.All()
	if all["OK"] != "fine" || all["DANGLING"] != "from-host" ||
		all["LOOP"] != "from-host" {
		t.Errorf("environment = %v", all)
	}
	for _, key := range []string{"DANGLING", "LOOP"} {
		origin, ok := result.Environment.Origin(key)
		if !ok || !origin.Winner.Host || len(origin.Shadowed) != 1 ||
			origin.Shadowed[0].Host {
			t.Errorf("%s origin = %+v, want host winner over namespace", key, origin)
		}
	}
	if _, ok := all["MISSING"]; ok {
		t.Error("MISSING should be omitted")
	}
	if _, ok := result.Environment.Origin("MISSING"); ok {
		t.Error("MISSING should have no origin")
	}

	want := []diagnostic.Finding{
		{Code: diagnostic.SecretReferenceIsUnresolved, Key: "DANGLING"},
		{Code: diagnostic.CircularVariableReference, Key: "LOOP"},
		{Code: diagnostic.UnresolvedVariableReference, Key: "MISSING"},
	}
	if len(result.Warnings) != len(want) {
		t.Fatalf("Warnings = %+v, want %d", result.Warnings, len(want))
	}
	for i, w := range result.Warnings {
		if w.Code != want[i].Code || w.Key != want[i].Key || w.Level != severity.Warn ||
			w.Project != "app" || w.Environment != "development" {
			t.Errorf("warning %d = %+v, want %+v", i, w, want[i])
		}
	}
	if !strings.Contains(result.Warnings[0].Message, "keeping the value") ||
		!strings.HasPrefix(result.Warnings[2].Message, "omitting MISSING") {
		t.Errorf("messages = %q / %q", result.Warnings[0].Message, result.Warnings[2].Message)
	}
}

// TestMaterializeCapturesHostOnce verifies one snapshot serves merging and the
// resolver within an operation.
func TestMaterializeCapturesHostOnce(t *testing.T) {
	t.Parallel()

	captures := 0
	opener := &fakeOpener{}
	s := newService(t, testWorkspace(), Dependencies{
		Namespaces: newFakeNamespaces().base(t, appInclude, "a: b"),
		HostEnv: func() hostenv.Snapshot {
			captures++
			return hostenv.From([]string{"X=1"})
		},
		OpenResolver: opener.open,
	})
	if _, err := s.Materialize(MaterializeParams{}); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if captures != 1 || opener.calls != 1 || !opener.reveal {
		t.Errorf(
			"captures = %d opens = %d reveal = %v",
			captures, opener.calls, opener.reveal,
		)
	}
	if v, _ := opener.host.Get("X"); v != "1" {
		t.Errorf("resolver host X = %q, want 1", v)
	}
}
