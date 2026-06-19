// Package runner handles executing child processes with environment injection,
// signal forwarding, and exit-code propagation. It is the final stage of the
// "envx run" pipeline: after environment resolution, this package spawns the
// child command with the assembled environment. It is domain-agnostic — it
// knows nothing about manifests or merging.
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

	"github.com/go-envx/envx/apps/envx/internal/shared/exitcode"
)

// -------------------------------------------------------------------------------------
// Options configures the runner's behavior for a single child execution.
type Options struct {
	// Env is the merged set of env vars to inject into the child process.
	Env map[string]string

	// Overload controls env-var precedence:
	//   false (default): existing OS env vars take priority over file values.
	//   true:            file values override existing OS env vars.
	Overload bool

	// Stdout and Stderr override where the child's output is written. When nil,
	// os.Stdout / os.Stderr are used (normal interactive mode). These are
	// configurable primarily for in-process testing.
	Stdout io.Writer
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------
// Run spawns the specified command as a child process with the merged
// environment (respecting Overload precedence), forwarding SIGINT and SIGTERM
// so the child can shut down gracefully, and propagating the child's exact exit
// code as an exitcode.Error. Returns nil on success, an exitcode.Error for a
// non-zero exit, or a wrapped error when the process fails to start.
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

	//nolint:gosec // subprocess execution is the explicit purpose of this package
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = buildEnv(opts)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	// Forward SIGINT/SIGTERM to the child for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case sig, ok := <-sigCh:
				if !ok {
					return
				}
				if cmd.Process != nil {
					_ = cmd.Process.Signal(sig)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	err := cmd.Wait()

	signal.Stop(sigCh)
	close(sigCh)
	<-done

	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &exitcode.Error{Code: exitErr.ExitCode()}
	}
	return fmt.Errorf("running command: %w", err)
}

// -------------------------------------------------------------------------------------
// buildEnv constructs the full environment slice for the child process by
// combining the OS environment with the merged file values. With Overload=false
// (default) OS env wins, so CI-set vars cannot be clobbered by checked-in
// files; with Overload=true file values win.
func buildEnv(opts Options) []string {
	var base, overlay map[string]string

	osEnv := envToMap(os.Environ())
	if opts.Overload {
		base, overlay = osEnv, opts.Env
	} else {
		base, overlay = opts.Env, osEnv
	}

	env := make(map[string]string, len(base)+len(overlay))
	maps.Copy(env, base)
	maps.Copy(env, overlay)
	return mapToEnv(env)
}

// -------------------------------------------------------------------------------------
// envToMap converts an os.Environ()-style slice (["KEY=VALUE", ...]) into a map
// for easy lookup and overlay operations.
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
