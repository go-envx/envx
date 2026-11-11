package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// maxStderr bounds the captured stderr kept for error reporting.
const maxStderr = 4 << 10

// waitDelay bounds how long Output waits for pipes after the child exits or is killed.
const waitDelay = time.Second

// OutputCommand specifies a captured subprocess invocation.
type OutputCommand struct {
	// Args holds the program and its arguments; no shell is involved.
	Args []string
	// Env is the complete child environment; nil or empty means none.
	Env map[string]string
}

// ExitError reports a child that ran but exited unsuccessfully.
type ExitError struct {
	// Code is the child's exit status.
	Code int
	// Stderr holds the first bytes the child wrote to stderr.
	Stderr string
}

// boundedBuffer keeps the first limit bytes written and discards the rest.
type boundedBuffer struct {
	// buf holds the retained bytes.
	buf bytes.Buffer
	// limit is the maximum number of retained bytes.
	limit int
}

// Output runs the command and returns its stdout once it exits successfully.
//
// Stdin is empty, and stderr is captured up to a fixed bound for ExitError.
// Cancelling ctx kills the child; the returned error then wraps ctx.Err().
// A missing program yields an error wrapping exec.ErrNotFound.
func Output(ctx context.Context, command OutputCommand) ([]byte, error) {
	if len(command.Args) == 0 {
		return nil, ErrNoCommand
	}

	var stdout bytes.Buffer
	stderr := &boundedBuffer{limit: maxStderr}

	//nolint:gosec // arguments are passed directly to the program, never a shell
	cmd := exec.CommandContext(ctx, command.Args[0], command.Args[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	cmd.Env = mapToEnv(command.Env)
	cmd.WaitDelay = waitDelay

	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("running %s: %w", command.Args[0], ctxErr)
	}
	if err == nil {
		return stdout.Bytes(), nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, &ExitError{
			Code:   exitCode(exitErr),
			Stderr: stderr.buf.String(),
		}
	}
	return nil, fmt.Errorf("running %s: %w", command.Args[0], err)
}

// Error describes the exit status without the captured stderr.
func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// Write retains bytes up to the limit and always reports full consumption.
func (b *boundedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
