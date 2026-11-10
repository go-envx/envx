package env

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// fakeRenderer records render calls and writes a fixed body.
type fakeRenderer struct {
	// validateErr is returned from Validate.
	validateErr error
	// project is the last rendered project.
	project string
	// entries are the last rendered entries.
	entries []ExplainEntry
}

// Validate implements Renderer.
func (r *fakeRenderer) Validate(EmitOptions) error {
	return r.validateErr
}

// Render implements Renderer.
func (r *fakeRenderer) Render(
	out io.Writer, project string, entries []ExplainEntry, _ EmitOptions,
) error {
	r.project, r.entries = project, entries
	_, err := io.WriteString(out, "rendered")
	return err
}

// emitFixture serves a namespace for emit with a fake renderer.
func emitFixture(
	t *testing.T, yaml string,
) (*Service, *fakeRenderer, *fakeNamespaces) {
	t.Helper()
	renderer := &fakeRenderer{}
	namespaces := newFakeNamespaces().base(t, appInclude, yaml)
	opener := &fakeOpener{plaintext: map[string]string{"secret://db/password": "s3cr3t"}}
	return newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    namespaces,
		HostEnv:          hostOf("HOST_ONLY=x"),
		NewValueResolver: opener.open,
		FormatRenderer:   renderer,
	}), renderer, namespaces
}

// TestEmitHostOverridesSecret verifies a host value replacing a secret is
// still rendered and reported as secret.
func TestEmitHostOverridesSecret(t *testing.T) {
	t.Parallel()

	renderer := &fakeRenderer{}
	s := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    newFakeNamespaces().base(t, appInclude, "password: secret://db/x"),
		HostEnv:          hostOf("PASSWORD=from-host"),
		NewValueResolver: (&fakeOpener{}).open,
		FormatRenderer:   renderer,
	})
	artifact, err := s.Emit(EmitParams{})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := renderer.entries[0]
	if !artifact.ContainsSecrets || !got.Secret || got.Evaluation.Value != "from-host" {
		t.Errorf("artifact = %+v, entry = %+v", artifact, got)
	}
}

// TestEmitSecretThroughCycle verifies secret derivation through a host-broken
// cycle does not depend on which key is evaluated first.
func TestEmitSecretThroughCycle(t *testing.T) {
	t.Parallel()

	renderer := &fakeRenderer{}
	opener := &fakeOpener{plaintext: map[string]string{"secret://db/password": "s3cr3t"}}
	s := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces: newFakeNamespaces().base(t, appInclude, `
			a: "{{B}}{{C}}"
			b: "{{A}}"
			c: secret://db/password
		`),
		HostEnv:          hostOf("A=from-host"),
		NewValueResolver: opener.open,
		FormatRenderer:   renderer,
	})
	if _, err := s.Emit(EmitParams{}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	for _, entry := range renderer.entries {
		if !entry.Secret {
			t.Errorf("%s Secret = false, want true", entry.Key)
		}
	}
}

// TestEmitIdentityValidatesLists verifies a list item containing the delimiter
// fails without a resolver, as materialization does.
func TestEmitIdentityValidatesLists(t *testing.T) {
	t.Parallel()

	renderer := &fakeRenderer{}
	s := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:  newFakeNamespaces().base(t, appInclude, `items: ["a,b", c]`),
		FormatRenderer: renderer,
	})
	if _, err := s.Emit(EmitParams{}); !errors.Is(err, ErrUnresolved) {
		t.Errorf("Emit error = %v, want ErrUnresolved", err)
	}
}

// TestEmit verifies revealed entries render for the resolved project and the
// artifact reports secret content per slice.
func TestEmit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// only selects the slice.
		only string
		// want reports whether the artifact contains secrets.
		want bool
	}{
		{only: "", want: true},
		{only: OnlySecrets, want: true},
		{only: OnlyConfig, want: false},
	}
	for _, tt := range tests {
		s, renderer, _ := emitFixture(t, `
			password: secret://db/password
			url: "x:{{PASSWORD}}"
			plain: p
		`)
		artifact, err := s.Emit(EmitParams{Options: EmitOptions{Only: tt.only}})
		if err != nil {
			t.Fatalf("Emit(%q): %v", tt.only, err)
		}
		if string(artifact.Data) != "rendered" || artifact.ContainsSecrets != tt.want {
			t.Errorf("Emit(%q) = %+v, want secrets %v", tt.only, artifact, tt.want)
		}
		if renderer.project != "app" || len(renderer.entries) != 3 {
			t.Errorf("rendered %q with %d entries", renderer.project, len(renderer.entries))
		}
		url := renderer.entries[2]
		if url.Key != "URL" || url.Evaluation.Value != "x:s3cr3t" || !url.Secret {
			t.Errorf("URL entry = %+v", url)
		}
	}
}

// TestEmitFailsClosed verifies option errors precede any read and unresolved
// values prevent rendering.
func TestEmitFailsClosed(t *testing.T) {
	t.Parallel()

	s, renderer, namespaces := emitFixture(t, "a: b")
	renderer.validateErr = errors.New("bad target")
	if _, err := s.Emit(EmitParams{}); !errors.Is(err, renderer.validateErr) {
		t.Errorf("Emit error = %v, want validation error", err)
	}
	if namespaces.baseReads != 0 {
		t.Errorf("base reads = %d before validation passed", namespaces.baseReads)
	}

	s, renderer, _ = emitFixture(t, `
		b: secret://db/missing
		a: "{{NOWHERE}}"
		c: [x, "y,z"]
		ok: fine
	`)
	_, err := s.Emit(EmitParams{})
	if !errors.Is(err, ErrUnresolved) || !strings.Contains(err.Error(), "3 value(s)") ||
		!strings.Contains(err.Error(), "A, B, C") {
		t.Errorf("Emit error = %v", err)
	}
	if renderer.entries != nil {
		t.Error("renderer called despite unresolved values")
	}

	noRenderer := newService(
		t, testWorkspace(), Dependencies{EnvNamespaces: newFakeNamespaces()},
	)
	if _, err := noRenderer.Emit(EmitParams{}); err == nil {
		t.Error("Emit without Renderer succeeded")
	}
}
