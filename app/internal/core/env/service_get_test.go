package env

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/workspace"
)

// getFixture serves a namespace with a secret, a template, and a broken key.
func getFixture(t *testing.T, host ...string) (*Service, *fakeOpener) {
	t.Helper()
	namespaces := newFakeNamespaces().base(t, appInclude, `
		password: secret://db/password
		host: db.local
		url: postgres://{{HOST}}:{{PASSWORD}}
		broken: secret://db/missing
		hosts: [a, b]
	`)
	opener := &fakeOpener{plaintext: map[string]string{"secret://db/password": "s3cr3t"}}
	s := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    namespaces,
		HostEnv:          hostOf(host...),
		NewValueResolver: opener.open,
	})
	return s, opener
}

// TestGet verifies masked and revealed reads of plain, secret, list, and
// templated values.
func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// key is the requested key.
		key string
		// reveal requests plaintext.
		reveal bool
		// want is the expected value.
		want string
	}{
		{key: "host", want: "db.local"},
		{key: "password", want: "secret://db/password"},
		{key: "password", reveal: true, want: "s3cr3t"},
		{key: "url", want: "postgres://{{HOST}}:{{PASSWORD}}"},
		{key: "url", reveal: true, want: "postgres://db.local:s3cr3t"},
		{key: "hosts", want: "a,b"},
	}
	for _, tt := range tests {
		s, opener := getFixture(t)
		got, err := s.Get(GetParams{Key: tt.key, Reveal: tt.reveal})
		if err != nil {
			t.Errorf("Get(%s, reveal=%v): %v", tt.key, tt.reveal, err)
			continue
		}
		if got.Value != tt.want || !strings.EqualFold(got.Key, tt.key) {
			t.Errorf("Get(%s, reveal=%v) = %+v, want %q", tt.key, tt.reveal, got, tt.want)
		}
		if opener.calls != 1 || opener.reveal != tt.reveal {
			t.Errorf("opener calls = %d reveal = %v", opener.calls, opener.reveal)
		}
	}
}

// TestGetFailures verifies missing keys and failing dependencies error without
// leaking values, while unrelated failures do not block.
func TestGetFailures(t *testing.T) {
	t.Parallel()

	s, _ := getFixture(t)
	if _, err := s.Get(GetParams{Key: "nope"}); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get(missing key) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := s.Get(GetParams{Key: "broken", Reveal: true}); err == nil {
		t.Error("Get(broken, reveal) succeeded")
	}

	namespaces := newFakeNamespaces().base(t, appInclude, `
		password: secret://db/password
		url: "{{PASSWORD}}"
		loop: "{{LOOP}}"
	`)
	opener := &fakeOpener{}
	failing := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces: namespaces, NewValueResolver: opener.open,
	})
	_, err := failing.Get(GetParams{Key: "url", Reveal: true})
	if err == nil || strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("dependency error = %v", err)
	}
	_, err = failing.Get(GetParams{Key: "loop", Reveal: true})
	if !errors.Is(err, ErrCircularReference) {
		t.Errorf("cycle error = %v, want ErrCircularReference", err)
	}
}

// TestGetHostSelection verifies host overrides, overload, and that host-only
// keys are not readable.
func TestGetHostSelection(t *testing.T) {
	t.Parallel()

	s, opener := getFixture(t, "HOST=from-host", "HOST_ONLY=x")
	got, err := s.Get(GetParams{Key: "host"})
	if err != nil || got.Value != "from-host" || !got.Origin.Winner.Host {
		t.Errorf("Get(host) = %+v, %v; want host winner", got, err)
	}
	if v, _ := opener.host.Get("HOST"); v != "from-host" {
		t.Errorf("resolver host snapshot HOST = %q, want from-host", v)
	}
	if _, err := s.Get(GetParams{Key: "host_only"}); err == nil {
		t.Error("Get(host-only key) succeeded")
	}

	got, err = s.Get(GetParams{
		Query: Query{Options: workspace.Settings{Overload: ptr(true)}},
		Key:   "host",
	})
	if err != nil || got.Value != "db.local" || got.Origin.Winner.Host {
		t.Errorf("Get(host, overload) = %+v, %v; want file winner", got, err)
	}
}

// TestGetWithoutResolver verifies a nil opener is identity resolution.
func TestGetWithoutResolver(t *testing.T) {
	t.Parallel()

	s := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces: newFakeNamespaces().base(t, appInclude, "password: secret://db/x"),
	})
	got, err := s.Get(GetParams{Key: "password", Reveal: true})
	if err != nil || got.Value != "secret://db/x" {
		t.Errorf("Get = %q, %v; want identity", got.Value, err)
	}
}
