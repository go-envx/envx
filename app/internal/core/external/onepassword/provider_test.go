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

	got, err := p.Fetch(context.Background(), "vault/item name/field")
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
			_, err := p.Fetch(context.Background(), "v/i/f")
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
