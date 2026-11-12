// Package onepassword resolves op:// references with the 1Password CLI.
package onepassword

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
)

// program is the 1Password CLI executable.
const program = "op"

// cliEnv selects the CLI's own variables, such as OP_SERVICE_ACCOUNT_TOKEN.
var cliEnv = regexp.MustCompile(`(?i)^OP_`)

// Options configures the provider.
type Options struct {
	// Run executes the CLI and returns its stdout; nil uses process.Output.
	Run external.CommandRunner
	// LookPath locates the CLI offline; nil uses exec.LookPath.
	LookPath func(file string) (string, error)
}

// Provider fetches op:// references with `op read`.
type Provider struct {
	// run executes the CLI and returns its stdout.
	run external.CommandRunner
	// lookPath locates the CLI offline.
	lookPath func(file string) (string, error)
	// env is the minimal child environment.
	env map[string]string
}

// New constructs a provider whose CLI runs with the host's base and OP_*
// variables only.
func New(host hostenv.Snapshot, opts Options) *Provider {
	run := opts.Run
	if run == nil {
		run = process.Output
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	return &Provider{
		run:      run,
		lookPath: lookPath,
		env:      process.MinimalEnv(host.All(), cliEnv),
	}
}

// Scheme returns "op".
func (p *Provider) Scheme() string {
	return external.SchemeOnePassword
}

// Qualify accepts "vault/item/field" and "vault/item/section/field" without a
// "#field" fragment; references need no qualification.
func (p *Provider) Qualify(
	ref external.Reference, _ string,
) (external.Reference, error) {
	if ref.Field != "" {
		return external.Reference{}, fmt.Errorf(
			"%w: op references do not support #<field>; "+
				"name the field in the path",
			external.ErrInvalidReference,
		)
	}
	if segments := strings.Count(ref.Path, "/") + 1; segments < 3 || segments > 4 {
		return external.Reference{}, fmt.Errorf(
			"%w: want op://<vault>/<item>/[<section>/]<field>",
			external.ErrInvalidReference,
		)
	}
	return ref, nil
}

// CheckAvailable reports a missing CLI. Sessions are not checked, since the
// desktop app can authorize the CLI on first use.
func (p *Provider) CheckAvailable() error {
	if _, err := p.lookPath(program); err != nil {
		return notFoundError()
	}
	return nil
}

// Fetch reads one secret reference, such as "op://vault/item/field".
func (p *Provider) Fetch(ctx context.Context, ref external.Reference) (string, error) {
	if _, err := p.Qualify(ref, ""); err != nil {
		return "", err
	}
	read := external.Reference{Scheme: external.SchemeOnePassword, Path: ref.Path}
	out, err := p.run(ctx, process.OutputCommand{
		Args: []string{program, "read", "--no-newline", read.String()},
		Env:  p.env,
	})
	if err == nil {
		return string(out), nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "", notFoundError()
	}
	if exitErr, ok := errors.AsType[*process.ExitError](err); ok {
		return "", fmt.Errorf(
			"%w: %s read: exit status %d: %s",
			external.ErrReferenceUnresolved, program, exitErr.Code,
			external.SummarizeStderr(exitErr.Stderr),
		)
	}
	return "", err
}

// notFoundError reports the CLI missing from PATH.
func notFoundError() error {
	return fmt.Errorf(
		"%w: %s CLI not found on PATH; install 1Password CLI",
		external.ErrProviderUnavailable, program,
	)
}
