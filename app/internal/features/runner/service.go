package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"

	"github.com/go-envx/envx/app/internal/shared/exitcode"
)

// ServiceParams provides stream dependencies to the process execution service.
type ServiceParams struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// Service supervises child process lifecycle, signal propagation, and exit status.
type Service struct {
	params ServiceParams
}

// NewService constructs a process execution domain service.
func NewService(params ServiceParams) *Service {
	if params.Stdout == nil {
		params.Stdout = os.Stdout
	}
	if params.Stderr == nil {
		params.Stderr = os.Stderr
	}
	if params.Stdin == nil {
		params.Stdin = os.Stdin
	}
	return &Service{params: params}
}

// Run executes the command with injected environment and relays received signals.
func (s *Service) Run(params RunParams) error {
	if len(params.Args) == 0 {
		return ErrNoCommandSpecified
	}

	// exec.Command (not CommandContext) is deliberate: envx stays transparent by
	// forwarding signals to the child and mirroring its exit status, so the child
	// owns its own shutdown. Binding it to a context would let cancellation
	// SIGKILL it out from under a graceful shutdown — the surprise we avoid here.
	//nolint:gosec,noctx // intentional; see comment above
	cmd := exec.Command(params.Args[0], params.Args[1:]...)
	cmd.Stdin = s.params.Stdin
	cmd.Stdout = s.params.Stdout
	cmd.Stderr = s.params.Stderr
	cmd.Env = mapToEnv(params.Env)

	if err := cmd.Start(); err != nil {
		// Mirror a shell: report the failure and exit with its conventional code
		// (127 not-found, 126 found-but-not-executable) rather than the generic
		// runtime code, so scripts wrapping `envx run` can branch on it.
		_, _ = fmt.Fprintf(s.params.Stderr, "envx: %v\n", err)
		ec := &exitcode.Error{Code: startFailureCode(err)}
		return fmt.Errorf("%w: %w", ErrProcessStartFailed, ec)
	}

	// Relay signals to the child until it exits. Installing these handlers also
	// keeps envx alive (rather than dying on the signal and orphaning the child)
	// so it can wait for the child and mirror its final status. Terminal signals
	// the tty already delivered to an interactive child are not re-forwarded.
	interactive := s.stdinIsTerminal()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, forwardedSignals...)
	defer signal.Stop(sigCh)

	waitDone := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				if shouldForward(sig, interactive) {
					_ = cmd.Process.Signal(sig)
				}
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

// stdinIsTerminal reports whether standard input is a character device, the
// lightweight, dependency-free proxy for "running attached to a terminal" that
// tells Run whether the tty will deliver SIGINT/SIGQUIT to the child directly. A
// non-tty character device such as /dev/null is a harmless false positive: at
// worst envx skips forwarding a SIGINT a supervisor would otherwise send itself.
func (s *Service) stdinIsTerminal() bool {
	if f, ok := s.params.Stdin.(*os.File); ok {
		info, err := f.Stat()
		if err != nil {
			return false
		}
		return info.Mode()&os.ModeCharDevice != 0
	}
	return false
}
