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

// prodListing is the fake API's password list 1988.
const prodListing = `[{"PasswordID":50,"Title":"db","Password":"prod"}]`

// listIDs maps the test environments to the fake API's lists.
var listIDs = map[string]int{"dev": 1990, "prod": 1988}

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

// ref builds a passwordstate reference from "path[#field]" without validating
// it.
func ref(raw string) external.Reference {
	path, field, _ := strings.Cut(raw, "#")
	return external.Reference{
		Scheme: external.SchemePasswordstate,
		Path:   path,
		Field:  field,
	}
}

// newServer serves password 42 and lists 1990 and 1988 under /pws and records
// each request URI.
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
			case r.URL.RequestURI() == "/pws/api/passwords/1988?QueryAll":
				_, _ = w.Write([]byte(prodListing))
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
	return newFieldProvider(server, host, "")
}

// newFieldProvider builds a provider against server that reads field by
// default.
func newFieldProvider(
	server *httptest.Server, host hostenv.Snapshot, field string,
) *passwordstate.Provider {
	return passwordstate.New(host, passwordstate.Options{
		URL:       server.URL + "/pws/",
		APIKeyEnv: "PW_KEY",
		ListIDs:   listIDs,
		Field:     field,
		Client:    server.Client(),
	})
}

// fetch qualifies raw for environment and fetches it.
func fetch(p *passwordstate.Provider, raw, environment string) (string, error) {
	qualified, err := p.Qualify(ref(raw), environment)
	if err != nil {
		return "", err
	}
	return p.Fetch(context.Background(), qualified)
}

func TestFetchFields(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newProvider(newServer(t, &seen), keyHost)

	for path, want := range map[string]string{
		"42":          "s3cret",
		"42#UserName": "admin",
		"42#username": "admin",
	} {
		got, err := fetch(p, path, "dev")
		if err != nil || got != want {
			t.Errorf("fetch(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
	if paths := seen.all(); paths[0] != "/pws/api/passwords/42" {
		t.Errorf("path = %q, want /pws/api/passwords/42", paths[0])
	}
}

func TestRejectsInvalidReference(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newProvider(newServer(t, &seen), keyHost)

	for _, path := range []string{"db/x", "x/1990", "bad%zz", "1990/bad%zz"} {
		_, err := fetch(p, path, "dev")
		if !errors.Is(err, external.ErrInvalidReference) {
			t.Errorf("fetch(%q) err = %v, want ErrInvalidReference", path, err)
		}
	}
	// Fetch accepts only qualified references.
	_, err := p.Fetch(context.Background(), ref("db"))
	if !errors.Is(err, external.ErrInvalidReference) {
		t.Errorf("Fetch(db) err = %v, want ErrInvalidReference", err)
	}
	if paths := seen.all(); len(paths) != 0 {
		t.Errorf("requests = %q, want none", paths)
	}
}

func TestQualifyOffline(t *testing.T) {
	t.Parallel()

	p := passwordstate.New(keyHost, passwordstate.Options{
		URL:       "https://pw.local",
		APIKeyEnv: "PW_KEY",
		ListIDs:   listIDs,
	})
	if err := p.CheckAvailable(); err != nil {
		t.Errorf("CheckAvailable = %v, want nil", err)
	}
	for _, tt := range []struct {
		raw  string
		env  string
		want string
	}{
		{raw: "42", env: "qa", want: "42"},
		{raw: "42#UserName", env: "qa", want: "42#UserName"},
		{raw: "7/db", env: "qa", want: "7/db"},
		{raw: "7/a/b#Title", env: "qa", want: "7/a/b#Title"},
		{raw: "db", env: "dev", want: "1990/db"},
		{raw: "db#UserName", env: "prod", want: "1988/db#UserName"},
		{raw: "a%2Fb", env: "dev", want: "1990/a%2Fb"},
	} {
		got, err := p.Qualify(ref(tt.raw), tt.env)
		if err != nil || got != ref(tt.want) {
			t.Errorf("Qualify(%q, %q) = %v, %v; want %s", tt.raw, tt.env, got, err, tt.want)
		}
	}
	_, err := p.Qualify(ref("db"), "qa")
	if !errors.Is(err, external.ErrInvalidReference) || !strings.Contains(
		err.Error(), `no passwordstate list-id configured for environment "qa"`,
	) {
		t.Errorf("Qualify(db, qa) err = %v, want missing list-id", err)
	}
}

func TestFetchTitles(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newProvider(newServer(t, &seen), keyHost)

	for _, tt := range []struct {
		path string
		env  string
		want string
	}{
		{path: "db", env: "dev", want: "s3cret"},
		{path: "db#username", env: "dev", want: "admin"},
		{path: "a%2Fb", env: "dev", want: "slash"},
		{path: "1990/a/b", env: "prod", want: "slash"},
		{path: "1990/db", env: "prod", want: "s3cret"},
		{path: "db", env: "prod", want: "prod"},
		{path: "42#UserName", env: "dev", want: "admin"},
	} {
		got, err := fetch(p, tt.path, tt.env)
		if err != nil || got != tt.want {
			t.Errorf("fetch(%q, %q) = %q, %v; want %q", tt.path, tt.env, got, err, tt.want)
		}
	}
	want := []string{
		"/pws/api/passwords/1990?QueryAll",
		"/pws/api/passwords/1988?QueryAll",
		"/pws/api/passwords/42",
	}
	if paths := seen.all(); !slices.Equal(paths, want) {
		t.Errorf("requests = %q, want %q", paths, want)
	}
}

func TestFetchTitleErrors(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newProvider(newServer(t, &seen), keyHost)

	for path, contain := range map[string]string{
		"missing": `no password titled "missing" in list 1990`,
		"DB":      `no password titled "DB" in list 1990`,
		"dup": `2 passwords titled "dup" in list 1990; ` +
			"reference one by ID, such as passwordstate://44 (IDs: 44, 45)",
		"db#Missing": `password "db" has no field "Missing"`,
	} {
		_, err := fetch(p, path, "dev")
		if !errors.Is(err, external.ErrReferenceUnresolved) ||
			!strings.Contains(err.Error(), contain) {
			t.Errorf("fetch(%q) err = %v, want containing %q", path, err, contain)
		}
		if strings.Contains(err.Error(), "s3cret") {
			t.Errorf("fetch(%q) err = %q, want values hidden", path, err)
		}
	}
}

func TestFetchDefaultField(t *testing.T) {
	t.Parallel()

	var seen requests
	p := newFieldProvider(newServer(t, &seen), keyHost, "UserName")

	for path, want := range map[string]string{
		"42":          "admin",
		"db":          "admin",
		"42#Password": "s3cret",
	} {
		got, err := fetch(p, path, "dev")
		if err != nil || got != want {
			t.Errorf("fetch(%q) = %q, %v; want %q", path, got, err, want)
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
			if err := p.CheckAvailable(); !errors.Is(err, external.ErrProviderUnavailable) {
				t.Errorf("CheckAvailable = %v, want ErrProviderUnavailable", err)
			}
			_, err := p.Fetch(context.Background(), ref("42"))
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
			path:    "42#Missing",
			contain: `password 42 has no field "Missing"`,
		},
		"non-text field": {
			host:    keyHost,
			path:    "42#PasswordID",
			contain: `field "PasswordID" of password 42 is not text`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := newProvider(server, tt.host).Fetch(context.Background(), ref(tt.path))
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

	_, err := p.Fetch(context.Background(), ref("42"))
	if !errors.Is(err, external.ErrProviderUnavailable) {
		t.Errorf("err = %v, want ErrProviderUnavailable", err)
	}
}
