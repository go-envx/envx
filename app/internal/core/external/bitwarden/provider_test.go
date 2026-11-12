package bitwarden_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/core/external/bitwarden"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
)

const (
	// projectID identifies the "app" project.
	projectID = "e325ea69-a3ab-4dff-836f-b02e013fe530"
	// secretID identifies the DB_PASSWORD secret.
	secretID = "be8e0ad8-d545-4017-a55a-b02f014d4158" //nolint:gosec // test ID
)

// jsonFlags trails every bws invocation.
var jsonFlags = []string{"--output", "json", "--color", "no"}

// tokenHost carries an access token.
var tokenHost = hostenv.From([]string{"BWS_ACCESS_TOKEN=0.token"})

// fakeRunner returns canned stdout per subcommand and records every command.
type fakeRunner struct {
	// outputs maps the space-joined arguments after "bws" to stdout.
	outputs map[string]string
	// err is returned for every command when set.
	err error
	// calls records every command in order.
	calls []process.OutputCommand
}

func (r *fakeRunner) run(
	_ context.Context, command process.OutputCommand,
) ([]byte, error) {
	r.calls = append(r.calls, command)
	if r.err != nil {
		return nil, r.err
	}
	args := command.Args[1 : len(command.Args)-len(jsonFlags)]
	out, ok := r.outputs[strings.Join(args, " ")]
	if !ok {
		return nil, &process.ExitError{Code: 1, Stderr: "unexpected command"}
	}
	return []byte(out), nil
}

// subcommands returns the recorded commands without program and flags.
func (r *fakeRunner) subcommands() []string {
	got := make([]string, len(r.calls))
	for i, c := range r.calls {
		got[i] = strings.Join(c.Args[1:len(c.Args)-len(jsonFlags)], " ")
	}
	return got
}

// ref builds a bws reference to path.
func ref(path string) external.Reference {
	return external.Reference{Scheme: external.SchemeBitwarden, Path: path}
}

func TestFetchBySecretID(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{
		"secret get " + secretID: `{"id":"` + secretID + `","key":"K","value":"pw"}`,
	}}
	host := hostenv.From([]string{
		"PATH=/bin",
		"BWS_ACCESS_TOKEN=0.token",
		"bws_server_url=https://vault.bitwarden.eu",
		"OP_SERVICE_ACCOUNT_TOKEN=leak",
	})
	p := bitwarden.New(host, bitwarden.Options{Run: runner.run})

	got, err := p.Fetch(context.Background(), ref(secretID))
	if err != nil || got != "pw" {
		t.Fatalf("Fetch = %q, %v; want pw", got, err)
	}
	wantArgs := append([]string{"bws", "secret", "get", secretID}, jsonFlags...)
	if !slices.Equal(runner.calls[0].Args, wantArgs) {
		t.Errorf("Args = %q, want %q", runner.calls[0].Args, wantArgs)
	}
	wantEnv := map[string]string{
		"PATH":             "/bin",
		"BWS_ACCESS_TOKEN": "0.token",
		"bws_server_url":   "https://vault.bitwarden.eu",
	}
	if !maps.Equal(runner.calls[0].Env, wantEnv) {
		t.Errorf("Env = %v, want %v", runner.calls[0].Env, wantEnv)
	}
}

func TestFetchByProjectNameListsOnce(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{
		"project list": `[
			{"id":"` + projectID + `","name":"app"},
			{"id":"00000000-0000-0000-0000-000000000000","name":"other"}
		]`,
		"secret list " + projectID: `[
			{"id":"` + secretID + `","key":"DB_PASSWORD","value":"pw"},
			{"id":"x","key":"api/token","value":"tok"}
		]`,
	}}
	p := bitwarden.New(tokenHost, bitwarden.Options{Run: runner.run})

	for _, tc := range []struct {
		// path is the reference path to fetch.
		path string
		// want is the expected plaintext.
		want string
	}{
		{path: "app/DB_PASSWORD", want: "pw"},
		{path: "app/api/token", want: "tok"},
		{path: projectID + "/DB_PASSWORD", want: "pw"},
	} {
		got, err := p.Fetch(context.Background(), ref(tc.path))
		if err != nil || got != tc.want {
			t.Errorf("Fetch(%q) = %q, %v; want %q", tc.path, got, err, tc.want)
		}
	}
	want := []string{"project list", "secret list " + projectID}
	if got := runner.subcommands(); !slices.Equal(got, want) {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

func TestFetchByProjectIDSkipsProjectList(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{
		"secret list " + projectID: `[{"id":"` + secretID + `","key":"K","value":"v"}]`,
	}}
	p := bitwarden.New(tokenHost, bitwarden.Options{Run: runner.run})

	got, err := p.Fetch(context.Background(), ref(projectID+"/K"))
	if err != nil || got != "v" {
		t.Fatalf("Fetch = %q, %v; want v", got, err)
	}
	want := []string{"secret list " + projectID}
	if got := runner.subcommands(); !slices.Equal(got, want) {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

func TestFetchLookupFailures(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{
		"project list": `[
			{"id":"` + projectID + `","name":"app"},
			{"id":"p1","name":"dup"},
			{"id":"p2","name":"dup"}
		]`,
		"secret list " + projectID: `[
			{"id":"s1","key":"TWICE","value":"a"},
			{"id":"s2","key":"TWICE","value":"b"}
		]`,
	}}
	tests := map[string]string{
		"missing/K":   `no accessible project named "missing"`,
		"dup/K":       `2 projects named "dup"; reference one by ID (IDs: p1, p2)`,
		"app/MISSING": `no secret with key "MISSING" in project "app"`,
		"app/TWICE":   "such as bws://s1 (IDs: s1, s2)",
	}
	p := bitwarden.New(tokenHost, bitwarden.Options{Run: runner.run})
	for path, contain := range tests {
		_, err := p.Fetch(context.Background(), ref(path))
		if !errors.Is(err, external.ErrReferenceUnresolved) {
			t.Errorf("Fetch(%q) err = %v, want ErrReferenceUnresolved", path, err)
			continue
		}
		if !strings.Contains(err.Error(), contain) {
			t.Errorf("Fetch(%q) err = %q, want to contain %q", path, err, contain)
		}
		if strings.Contains(err.Error(), `"a"`) || strings.Contains(err.Error(), `"b"`) {
			t.Errorf("Fetch(%q) err = %q, want no secret values", path, err)
		}
	}
}

func TestFetchRejectsInvalidReference(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	p := bitwarden.New(tokenHost, bitwarden.Options{Run: runner.run})
	field := ref(secretID)
	field.Field = "value"

	for _, r := range []external.Reference{ref("DB_PASSWORD"), field} {
		if _, err := p.Fetch(context.Background(), r); !errors.Is(
			err, external.ErrInvalidReference,
		) {
			t.Errorf("Fetch(%s) err = %v, want ErrInvalidReference", r, err)
		}
	}
	if len(runner.calls) != 0 {
		t.Errorf("calls = %q, want none", runner.subcommands())
	}
}

func TestQualify(t *testing.T) {
	t.Parallel()

	p := bitwarden.New(hostenv.Snapshot{}, bitwarden.Options{})
	field := ref("app/KEY")
	field.Field = "value"
	for _, tt := range []struct {
		ref   external.Reference
		valid bool
	}{
		{ref(secretID), true},
		{ref("app/KEY"), true},
		{ref("DB_PASSWORD"), false},
		{field, false},
	} {
		got, err := p.Qualify(tt.ref, "dev")
		if valid := err == nil; valid != tt.valid || valid && got != tt.ref ||
			err != nil && !errors.Is(err, external.ErrInvalidReference) {
			t.Errorf("Qualify(%s) = %v, %v; want valid %t", tt.ref, got, err, tt.valid)
		}
	}
}

func TestCheckAvailable(t *testing.T) {
	t.Parallel()

	found := func(string) (string, error) { return "/bin/bws", nil }
	missing := func(string) (string, error) { return "", exec.ErrNotFound }
	tests := map[string]struct {
		host     hostenv.Snapshot
		lookPath func(string) (string, error)
		contain  string
	}{
		"available": {host: tokenHost, lookPath: found},
		"no token": {
			host:     hostenv.From([]string{"BWS_ACCESS_TOKEN="}),
			lookPath: found,
			contain:  "BWS_ACCESS_TOKEN is not set",
		},
		"no cli": {
			host:     tokenHost,
			lookPath: missing,
			contain:  "install Bitwarden Secrets Manager CLI",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := bitwarden.New(tt.host, bitwarden.Options{LookPath: tt.lookPath})
			err := p.CheckAvailable()
			if tt.contain == "" {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, external.ErrProviderUnavailable) ||
				!strings.Contains(err.Error(), tt.contain) {
				t.Errorf("err = %v, want containing %q", err, tt.contain)
			}
		})
	}
}

func TestFetchRequiresToken(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	host := hostenv.From([]string{"BWS_ACCESS_TOKEN="})
	p := bitwarden.New(host, bitwarden.Options{Run: runner.run})

	_, err := p.Fetch(context.Background(), ref(secretID))
	if !errors.Is(err, external.ErrProviderUnavailable) {
		t.Errorf("err = %v, want ErrProviderUnavailable", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("calls = %q, want none", runner.subcommands())
	}
}

func TestFetchCommandErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err     error
		out     string
		want    error
		contain string
	}{
		"missing cli": {
			err:     fmt.Errorf("running bws: %w", exec.ErrNotFound),
			want:    external.ErrProviderUnavailable,
			contain: "install Bitwarden Secrets Manager CLI",
		},
		"exit": {
			err: &process.ExitError{
				Code:   1,
				Stderr: "Error: Resource not found\nsecond line",
			},
			want:    external.ErrReferenceUnresolved,
			contain: "bws secret get: exit status 1: Error: Resource not found",
		},
		"exit with cause": {
			err: &process.ExitError{
				Code:   1,
				Stderr: "Error: \n   0: Resource not found.\n\nsecond line",
			},
			want:    external.ErrReferenceUnresolved,
			contain: "bws secret get: exit status 1: Resource not found.",
		},
		"malformed output": {
			out:     `not json "secret-value"`,
			want:    external.ErrReferenceUnresolved,
			contain: "unexpected output format",
		},
		"canceled": {
			err:  context.Canceled,
			want: context.Canceled,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{
				err:     tt.err,
				outputs: map[string]string{"secret get " + secretID: tt.out},
			}
			p := bitwarden.New(tokenHost, bitwarden.Options{Run: runner.run})
			_, err := p.Fetch(context.Background(), ref(secretID))
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.contain) {
				t.Errorf("err = %q, want to contain %q", err, tt.contain)
			}
			if strings.Contains(err.Error(), "second line") ||
				strings.Contains(err.Error(), "secret-value") {
				t.Errorf("err = %q, want stderr truncated and stdout hidden", err)
			}
		})
	}
}
