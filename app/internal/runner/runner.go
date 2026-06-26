package runner

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/go-envx/envx/app/internal/exitcode"
)

// -------------------------------------------------------------------------------------

// Run spawns the specified command as a child process with the merged
// environment (respecting Overload precedence), forwarding SIGINT and SIGTERM
// so the child can shut down gracefully, and propagating the child's exact exit
// code as an *exitcode.Error. Returns nil on success, an *exitcode.Error for a
// non-zero exit, or a wrapped error when the process fails to start.
func Run(ctx context.Context, args []string, p Params) error {
	if len(args) == 0 {
		return errors.New("no command specified")
	}

	normalizeParams(&p)

	//nolint:gosec // subprocess execution is the explicit purpose of this package
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Env = buildEnv(p)

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

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return &exitcode.Error{Code: exitErr.ExitCode()}
	}
	return fmt.Errorf("running command: %w", err)
}

// -------------------------------------------------------------------------------------

// buildEnv constructs the full environment slice for the child process by
// combining the OS environment with the merged file values. With Overload=false
// (default) OS env wins, so CI-set vars cannot be clobbered by checked-in
// files; with Overload=true file values win.
func buildEnv(p Params) []string {
	var base, overlay map[string]string

	osEnv := envToMap(os.Environ())
	if p.Overload {
		base, overlay = osEnv, p.Env
	} else {
		base, overlay = p.Env, osEnv
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
