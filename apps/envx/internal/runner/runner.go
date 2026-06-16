// Package runner handles executing child processes with environment injection,
// signal forwarding, and exit code propagation. It is the final stage of the
// "envx run" pipeline: after manifest resolution and env merging, this package
// spawns the child command with the assembled environment.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/go-envx/envx/apps/envx/internal/exitcode"
)

// -------------------------------------------------------------------------------------
// Options configures the runner's behavior for a single child execution.
type Options struct {
	// Env is the merged set of env vars to inject into the child process.
	Env map[string]string

	// Overload controls env var precedence:
	//   false (default): existing OS env vars take priority over file values.
	//   true:            file values override existing OS env vars.
	Overload bool

	// Dir is the working directory for the child process. Empty inherits cwd.
	Dir string

	// Stdout and Stderr override where the child's output is written.
	// When nil, os.Stdout / os.Stderr are used (normal interactive mode).
	// These are configurable primarily for in-process testing.
	Stdout io.Writer
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------
// Run spawns the specified command as a child process with:
//   - The merged environment (respecting Overload precedence)
//   - Signal forwarding: SIGINT and SIGTERM are relayed to the child so it can
//     perform graceful shutdown
//   - Exit code propagation: the child's exact exit code is surfaced as an
//     exitcode.Error so the parent can exit with the same code
//
// Returns nil on success (exit 0), exitcode.Error for non-zero exits, or a
// wrapped error for failures to start the process.
func Run(ctx context.Context, args []string, opts Options) error {
	if len(args) == 0 {
		return errors.New("no command specified")
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	//nolint:gosec // subprocess execution is the explicit purpose of this function
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = opts.Dir
	cmd.Env = buildEnv(opts)

	// Start the child process.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	// Forward signals to the child process. We catch SIGINT and SIGTERM
	// and relay them so the child can handle graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Signal forwarding goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case sig, ok := <-sigCh:
				if !ok {
					// Channel closed after cmd.Wait — exit cleanly.
					return
				}
				if cmd.Process != nil {
					_ = cmd.Process.Signal(sig)
				}
			case <-ctx.Done():
				// Context cancelled — the child will be killed by CommandContext.
				return
			}
		}
	}()

	// Wait for the child to exit.
	err := cmd.Wait()

	// Stop the signal forwarding goroutine and wait for it to exit.
	signal.Stop(sigCh)
	close(sigCh)
	<-done

	if err == nil {
		return nil
	}

	// Extract exit code from the error.
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return &exitcode.Error{Code: exitErr.ExitCode()}
	}
	return fmt.Errorf("running command: %w", err)
}

// -------------------------------------------------------------------------------------
// buildEnv constructs the full environment slice for the child process by
// combining the OS environment with the merged file values.
//
// Precedence depends on the Overload flag:
//   - Overload=false (default): OS env wins. File values are the base layer,
//     OS env vars overlay on top. This means CI-set vars cannot be accidentally
//     overwritten by checked-in files.
//   - Overload=true: File values win. OS env is the base layer, file values
//     overlay on top. Matches dotenvx's --overload behavior.
func buildEnv(opts Options) []string {
	var base, overlay map[string]string

	// Determine precedence: overlay wins over base for duplicate keys.
	//   Overload=true:  file values override OS env.
	//   Overload=false: OS env overrides file values (default, safe for CI).
	osEnv := envToMap(os.Environ())
	if opts.Overload {
		base, overlay = osEnv, opts.Env
	} else {
		base, overlay = opts.Env, osEnv
	}

	// Merge into a fresh map so neither input is mutated.
	env := make(map[string]string, len(base)+len(overlay))
	maps.Copy(env, base)
	maps.Copy(env, overlay)
	return mapToEnv(env)
}

// -------------------------------------------------------------------------------------
// envToMap converts an os.Environ()-style slice (["KEY=VALUE", ...]) into a
// map for easy lookup and overlay operations.
func envToMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, entry := range environ {
		for i := range entry {
			if entry[i] == '=' {
				m[entry[:i]] = entry[i+1:]
				break
			}
		}
	}
	return m
}

// -------------------------------------------------------------------------------------
// mapToEnv converts a key-value map back into the os.Environ() slice format
// (["KEY=VALUE", ...]) suitable for exec.Cmd.Env.
func mapToEnv(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, k+"="+v)
	}
	return result
}
