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

// maxStderr bounds the CLI diagnostic quoted in errors.
const maxStderr = 200

// cliEnv selects the CLI's own variables, such as OP_SERVICE_ACCOUNT_TOKEN.
var cliEnv = regexp.MustCompile(`(?i)^OP_`)

// Provider fetches op:// references with `op read`.
type Provider struct {
	// run executes the CLI and returns its stdout.
	run external.CommandRunner
	// env is the minimal child environment.
	env map[string]string
}

// New constructs a provider whose CLI runs with the host's base and OP_*
// variables only.
func New(run external.CommandRunner, host hostenv.Snapshot) *Provider {
	return &Provider{run: run, env: process.MinimalEnv(host.All(), cliEnv)}
}

// Scheme returns "op".
func (p *Provider) Scheme() string {
	return external.SchemeOnePassword
}

// Fetch reads one secret reference path, such as "vault/item/field".
func (p *Provider) Fetch(ctx context.Context, path string) (string, error) {
	ref := external.Reference{Scheme: external.SchemeOnePassword, Path: path}
	out, err := p.run(ctx, process.OutputCommand{
		Args: []string{program, "read", "--no-newline", ref.String()},
		Env:  p.env,
	})
	if err == nil {
		return string(out), nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf(
			"%w: %s CLI not found on PATH; install 1Password CLI",
			external.ErrProviderUnavailable, program,
		)
	}
	if exitErr, ok := errors.AsType[*process.ExitError](err); ok {
		return "", fmt.Errorf(
			"%w: %s read: exit status %d: %s",
			external.ErrReferenceUnresolved, program, exitErr.Code,
			summarize(exitErr.Stderr),
		)
	}
	return "", err
}

// summarize returns the first line of stderr, truncated.
func summarize(stderr string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
	line = strings.TrimSpace(line)
	if runes := []rune(line); len(runes) > maxStderr {
		return string(runes[:maxStderr]) + "..."
	}
	if line == "" {
		return "no diagnostic output"
	}
	return line
}
