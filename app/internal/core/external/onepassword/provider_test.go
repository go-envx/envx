package onepassword_test

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
	"github.com/go-envx/envx/app/internal/core/external/onepassword"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
)

// fakeRunner returns canned output and records the command it received.
type fakeRunner struct {
	// out is the stdout returned on success.
	out string
	// err is returned when set.
	err error
	// got records the last command.
	got process.OutputCommand
}

func (r *fakeRunner) run(
	_ context.Context, command process.OutputCommand,
) ([]byte, error) {
	r.got = command
	if r.err != nil {
		return nil, r.err
	}
	return []byte(r.out), nil
}

// ref builds an op reference to path.
func ref(path string) external.Reference {
	return external.Reference{Scheme: external.SchemeOnePassword, Path: path}
}

func TestFetchReadsReference(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{out: "pw"}
	host := hostenv.From([]string{
		"Path=/bin",
		"HOME=/home/u",
		"https_proxy=http://proxy:3128",
		"OP_SERVICE_ACCOUNT_TOKEN=ops_x",
		"op_session_acct=s",
		"AWS_SECRET_ACCESS_KEY=leak",
		"BWS_ACCESS_TOKEN=other",
	})
	p := onepassword.New(host, onepassword.Options{Run: runner.run})

	got, err := p.Fetch(context.Background(), ref("vault/item name/field"))
	if err != nil || got != "pw" {
		t.Fatalf("Fetch = %q, %v; want pw", got, err)
	}
	wantArgs := []string{"op", "read", "--no-newline", "op://vault/item name/field"}
	if !slices.Equal(runner.got.Args, wantArgs) {
		t.Errorf("Args = %q, want %q", runner.got.Args, wantArgs)
	}
	wantEnv := map[string]string{
		"Path":                     "/bin",
		"HOME":                     "/home/u",
		"https_proxy":              "http://proxy:3128",
		"OP_SERVICE_ACCOUNT_TOKEN": "ops_x",
		"op_session_acct":          "s",
	}
	if !maps.Equal(runner.got.Env, wantEnv) {
		t.Errorf("Env = %v, want %v", runner.got.Env, wantEnv)
	}
}

func TestQualify(t *testing.T) {
	t.Parallel()

	p := onepassword.New(hostenv.Snapshot{}, onepassword.Options{})
	field := ref("vault/item/field")
	field.Field = "x"
	for _, tt := range []struct {
		ref   external.Reference
		valid bool
	}{
		{ref("vault/item"), false},
		{ref("vault/item/field"), true},
		{ref("vault/item/sec/field"), true},
		{ref("vault/item/a/b/field"), false},
		{field, false},
	} {
		got, err := p.Qualify(tt.ref, "dev")
		if valid := err == nil; valid != tt.valid || valid && got != tt.ref ||
			err != nil && !errors.Is(err, external.ErrInvalidReference) {
			t.Errorf("Qualify(%s) = %v, %v; want valid %t", tt.ref, got, err, tt.valid)
		}
	}
}

func TestFetchRejectsField(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{out: "pw"}
	p := onepassword.New(hostenv.Snapshot{}, onepassword.Options{Run: runner.run})
	field := ref("vault/item/field")
	field.Field = "x"

	if _, err := p.Fetch(context.Background(), field); !errors.Is(
		err, external.ErrInvalidReference,
	) {
		t.Errorf("err = %v, want ErrInvalidReference", err)
	}
	if runner.got.Args != nil {
		t.Errorf("ran %q, want no command", runner.got.Args)
	}
}

func TestCheckAvailable(t *testing.T) {
	t.Parallel()

	found := func(string) (string, error) { return "/bin/op", nil }
	missing := func(string) (string, error) { return "", exec.ErrNotFound }

	p := onepassword.New(hostenv.Snapshot{}, onepassword.Options{LookPath: found})
	if err := p.CheckAvailable(); err != nil {
		t.Errorf("found: err = %v, want nil", err)
	}
	p = onepassword.New(hostenv.Snapshot{}, onepassword.Options{LookPath: missing})
	if err := p.CheckAvailable(); !errors.Is(err, external.ErrProviderUnavailable) ||
		!strings.Contains(err.Error(), "install 1Password CLI") {
		t.Errorf("missing: err = %v, want install hint", err)
	}
}

func TestFetchErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err     error
		want    error
		contain string
	}{
		"missing cli": {
			err:     fmt.Errorf("running op: %w", exec.ErrNotFound),
			want:    external.ErrProviderUnavailable,
			contain: "install 1Password CLI",
		},
		"exit": {
			err: &process.ExitError{
				Code:   1,
				Stderr: "[ERROR] could not read secret\nsecond line",
			},
			want:    external.ErrReferenceUnresolved,
			contain: "exit status 1: [ERROR] could not read secret",
		},
		"truncated stderr": {
			err:     &process.ExitError{Code: 1, Stderr: strings.Repeat("x", 500)},
			want:    external.ErrReferenceUnresolved,
			contain: strings.Repeat("x", 200) + "...",
		},
		"canceled": {
			err:  context.Canceled,
			want: context.Canceled,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{err: tt.err}
			p := onepassword.New(hostenv.Snapshot{}, onepassword.Options{Run: runner.run})
			_, err := p.Fetch(context.Background(), ref("v/i/f"))
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.contain) {
				t.Errorf("err = %q, want to contain %q", err, tt.contain)
			}
			if strings.Contains(err.Error(), "second line") ||
				strings.Contains(err.Error(), strings.Repeat("x", 201)) {
				t.Errorf("err = %q, want stderr truncated", err)
			}
		})
	}
}
