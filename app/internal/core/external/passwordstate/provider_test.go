package passwordstate_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/core/external/passwordstate"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// apiKey is the key served to the fake API.
const apiKey = "test-key" //nolint:gosec // test credential

// record is the fake API's password 42.
const record = `[{"PasswordID":42,"Title":"db","UserName":"admin",` +
	`"Password":"s3cret","GenericField1":"","Notes":null}]`

// listing is the fake API's password list 1990.
const listing = `[{"PasswordID":42,"Title":"db","UserName":"admin",` +
	`"Password":"s3cret"},` +
	`{"PasswordID":43,"Title":"a/b","Password":"slash"},` +
	`{"PasswordID":44,"Title":"dup","Password":"one"},` +
	`{"PasswordID":45,"Title":"dup","Password":"two"}]`

// keyHost carries the API key.
var keyHost = hostenv.From([]string{"PW_KEY=" + apiKey})

// requests records the paths a fake API served, safely across goroutines.
type requests struct {
	// mu guards paths.
	mu sync.Mutex
	// paths lists each request path in order.
	paths []string
}

func (r *requests) add(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, path)
}

func (r *requests) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

// newServer serves password 42 and list 1990 under /pws and records each
// request URI.
func newServer(t *testing.T, seen *requests) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			seen.add(r.URL.RequestURI())
			switch {
			case r.Header.Get("APIKey") != apiKey:
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`[{"errors":[{"message":"Invalid API key"}]}]`))
			case r.URL.RequestURI() == "/pws/api/passwords/1990?QueryAll":
				_, _ = w.Write([]byte(listing))
			case r.URL.Path == "/pws/api/passwords/42":
				_, _ = w.Write([]byte(record))
			case r.URL.Path == "/pws/api/passwords/7":
				_, _ = w.Write([]byte(`[]`))
			case r.URL.Path == "/pws/api/passwords/8":
				_, _ = w.Write([]byte(`<html>s3cret</html>`))
			default:
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`[{"errors":[{"message":"Not found\nmore"}]}]`))
			}
		},
	))
	t.Cleanup(server.Close)
	return server
}

// newProvider builds a provider against server with the given host.
func newProvider(
	server *httptest.Server, host hostenv.Snapshot,
) *passwordstate.Provider {
	return newListProvider(server, host, 0, "")
}

// newListProvider builds a provider against server that searches listID by
// title and reads field by default.
func newListProvider(
	server *httptest.Server, host hostenv.Snapshot, listID int, field string,
) *passwordstate.Provider {
	return passwordstate.New(host, passwordstate.Options{
		URL:       server.URL + "/pws/",
		APIKeyEnv: "PW_KEY",
		ListID:    listID,
		Field:     field,
		Client:    server.Client(),
	})
}

func TestFetchFields(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newProvider(newServer(t, &seen), keyHost)

	for path, want := range map[string]string{
		"42":          "s3cret",
		"42/UserName": "admin",
		"42/username": "admin",
	} {
		got, err := p.Fetch(context.Background(), path)
		if err != nil || got != want {
			t.Errorf("Fetch(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
	if paths := seen.all(); paths[0] != "/pws/api/passwords/42" {
		t.Errorf("path = %q, want /pws/api/passwords/42", paths[0])
	}
}

func TestFetchRejectsInvalidReference(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newProvider(newServer(t, &seen), keyHost)

	for _, path := range []string{"abc", "42/", "42/a/b", "../42", "bad%zz"} {
		_, err := p.Fetch(context.Background(), path)
		if !errors.Is(err, external.ErrInvalidReference) {
			t.Errorf("Fetch(%q) err = %v, want ErrInvalidReference", path, err)
		}
	}
	if paths := seen.all(); len(paths) != 0 {
		t.Errorf("requests = %q, want none", paths)
	}
}

func TestFetchTitleRequiresListID(t *testing.T) {
	t.Parallel()

	var seen requests
	_, err := newProvider(newServer(t, &seen), keyHost).
		Fetch(context.Background(), "db")
	if !errors.Is(err, external.ErrInvalidReference) ||
		!strings.Contains(err.Error(), "title references require list-id") {
		t.Errorf("err = %v, want list-id requirement", err)
	}
}

func TestFetchTitles(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newListProvider(newServer(t, &seen), keyHost, 1990, "")

	for _, tt := range []struct {
		path string
		want string
	}{
		{path: "db", want: "s3cret"},
		{path: "db/username", want: "admin"},
		{path: "a%2Fb", want: "slash"},
		{path: "42/UserName", want: "admin"},
	} {
		got, err := p.Fetch(context.Background(), tt.path)
		if err != nil || got != tt.want {
			t.Errorf("Fetch(%q) = %q, %v; want %q", tt.path, got, err, tt.want)
		}
	}
	want := []string{"/pws/api/passwords/1990?QueryAll", "/pws/api/passwords/42"}
	if paths := seen.all(); !slices.Equal(paths, want) {
		t.Errorf("requests = %q, want %q", paths, want)
	}
}

func TestFetchTitleErrors(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newListProvider(newServer(t, &seen), keyHost, 1990, "")

	for path, contain := range map[string]string{
		"missing": `no password titled "missing" in list 1990`,
		"DB":      `no password titled "DB" in list 1990`,
		"dup": `2 passwords titled "dup" in list 1990; ` +
			"reference one by ID, such as passwordstate://44 (IDs: 44, 45)",
		"db/Missing": `password "db" has no field "Missing"`,
	} {
		_, err := p.Fetch(context.Background(), path)
		if !errors.Is(err, external.ErrReferenceUnresolved) ||
			!strings.Contains(err.Error(), contain) {
			t.Errorf("Fetch(%q) err = %v, want containing %q", path, err, contain)
		}
		if strings.Contains(err.Error(), "s3cret") {
			t.Errorf("Fetch(%q) err = %q, want values hidden", path, err)
		}
	}
}

func TestFetchDefaultField(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newListProvider(newServer(t, &seen), keyHost, 1990, "UserName")

	for _, path := range []string{"42", "db"} {
		got, err := p.Fetch(context.Background(), path)
		if err != nil || got != "admin" {
			t.Errorf("Fetch(%q) = %q, %v; want admin", path, got, err)
		}
	}
}

func TestFetchUnavailable(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		opts    passwordstate.Options
		host    hostenv.Snapshot
		contain string
	}{
		"missing url": {
			opts:    passwordstate.Options{APIKeyEnv: "PW_KEY"},
			host:    keyHost,
			contain: "url is not configured",
		},
		"plain http": {
			opts:    passwordstate.Options{URL: "http://pw.local", APIKeyEnv: "PW_KEY"},
			host:    keyHost,
			contain: "must be an https URL",
		},
		"missing api-key-env": {
			opts:    passwordstate.Options{URL: "https://pw.local"},
			host:    keyHost,
			contain: "api-key-env is not configured",
		},
		"unset key": {
			opts:    passwordstate.Options{URL: "https://pw.local", APIKeyEnv: "PW_KEY"},
			host:    hostenv.From([]string{"PW_KEY="}),
			contain: "PW_KEY is not set",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := passwordstate.New(tt.host, tt.opts)
			_, err := p.Fetch(context.Background(), "42")
			if !errors.Is(err, external.ErrProviderUnavailable) {
				t.Fatalf("err = %v, want ErrProviderUnavailable", err)
			}
			if !strings.Contains(err.Error(), tt.contain) {
				t.Errorf("err = %q, want to contain %q", err, tt.contain)
			}
		})
	}
}

func TestFetchResponseErrors(t *testing.T) {
	t.Parallel()

	var seen requests
	server := newServer(t, &seen)
	wrongKey := hostenv.From([]string{"PW_KEY=wrong"})
	tests := map[string]struct {
		host    hostenv.Snapshot
		path    string
		contain string
	}{
		"unauthorized": {
			host:    wrongKey,
			path:    "42",
			contain: "GET password 42: HTTP 401: Invalid API key",
		},
		"not found": {
			host:    keyHost,
			path:    "9",
			contain: "HTTP 404: Not found",
		},
		"no record": {
			host:    keyHost,
			path:    "7",
			contain: "want 1 record, got 0",
		},
		"malformed": {
			host:    keyHost,
			path:    "8",
			contain: "unexpected response format",
		},
		"missing field": {
			host:    keyHost,
			path:    "42/Missing",
			contain: `password 42 has no field "Missing"`,
		},
		"non-text field": {
			host:    keyHost,
			path:    "42/PasswordID",
			contain: `field "PasswordID" of password 42 is not text`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := newProvider(server, tt.host).Fetch(context.Background(), tt.path)
			if !errors.Is(err, external.ErrReferenceUnresolved) {
				t.Fatalf("err = %v, want ErrReferenceUnresolved", err)
			}
			if !strings.Contains(err.Error(), tt.contain) {
				t.Errorf("err = %q, want to contain %q", err, tt.contain)
			}
			if strings.Contains(err.Error(), "s3cret") ||
				strings.Contains(err.Error(), "more") {
				t.Errorf("err = %q, want body hidden and message truncated", err)
			}
		})
	}
}

func TestFetchRequestFailure(t *testing.T) {
	t.Parallel()

	var seen requests
	server := newServer(t, &seen)
	// The default client rejects the test server's self-signed certificate.
	p := passwordstate.New(keyHost, passwordstate.Options{
		URL:       server.URL,
		APIKeyEnv: "PW_KEY",
	})

	_, err := p.Fetch(context.Background(), "42")
	if !errors.Is(err, external.ErrProviderUnavailable) {
		t.Errorf("err = %v, want ErrProviderUnavailable", err)
	}
}
