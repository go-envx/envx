package runner

import (
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

// forwardedSignals are the signals Run relays to the child. When envx runs in a
// terminal foreground, the tty already delivers SIGINT/SIGQUIT to the child
// directly; relaying additionally covers the case where a supervisor (systemd,
// docker) directs SIGTERM/SIGHUP at envx alone. The same signal is always
// forwarded unchanged, so a child is never sent a signal the user did not.
var forwardedSignals = []os.Signal{
	os.Interrupt,    // SIGINT  — Ctrl+C
	syscall.SIGTERM, // supervisors' default stop signal
	syscall.SIGHUP,  // controlling terminal closed
	syscall.SIGQUIT, // Ctrl+\
}

// -------------------------------------------------------------------------------------

// Run spawns the specified command as a child process with the merged
// environment (respecting Overload precedence). It stays deliberately
// transparent: rather than binding the child's lifetime to a context and
// force-killing it, Run relays received signals to the child and lets the child
// decide when to exit, then mirrors its exit status — so `envx run -- cmd`
// behaves just like running `cmd` directly. Returns nil on success, an
// *exitcode.Error carrying the child's code (128+signum when the child was
// terminated by a signal) so main.go can propagate it, or a wrapped error when
// the process fails to start.
func Run(args []string, p Params) error {
	if len(args) == 0 {
		return errors.New("no command specified")
	}

	normalizeParams(&p)

	// exec.Command (not CommandContext) is deliberate: envx stays transparent by
	// forwarding signals to the child and mirroring its exit status, so the child
	// owns its own shutdown. Binding it to a context would let cancellation
	// SIGKILL it out from under a graceful shutdown — the surprise we avoid here.
	//nolint:gosec,noctx // intentional; see comment above
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Env = buildEnv(p)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	// Relay signals to the child until it exits. Installing these handlers also
	// keeps envx alive (rather than dying on the signal and orphaning the child)
	// so it can wait for the child and mirror its final status.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, forwardedSignals...)
	defer signal.Stop(sigCh)

	waitDone := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				_ = cmd.Process.Signal(sig)
			case <-waitDone:
				return
			}
		}
	}()

	err := cmd.Wait()
	close(waitDone)

	if err == nil {
		return nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return &exitcode.Error{Code: exitCode(exitErr)}
	}
	return fmt.Errorf("running command: %w", err)
}

// -------------------------------------------------------------------------------------

// exitCode extracts the child's exit code, mapping a signal-terminated child to
// the shell convention 128+signum (e.g. 130 for SIGINT) instead of the -1 that
// ExitCode reports for a signaled process.
func exitCode(exitErr *exec.ExitError) int {
	if code := exitErr.ExitCode(); code >= 0 {
		return code
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return 1
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
