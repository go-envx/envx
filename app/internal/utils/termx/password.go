package termx

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// PasswordParams specifies the prompt, output, and input file descriptor for
// reading hidden password input.
type PasswordParams struct {
	// Out receives the prompt and trailing newline. If nil and Prompt is
	// non-empty, defaults to os.Stderr.
	Out io.Writer
	// In supplies the terminal file descriptor. If nil, defaults to os.Stdin.
	In *os.File
	// Prompt is written to Out before reading hidden input.
	Prompt string
	// ReadPassword reads one value from a terminal with echo disabled.
	// Defaults to term.ReadPassword when nil.
	ReadPassword func(int) ([]byte, error)
}

// Password prompts the user with the given prompt and reads a hidden value
// with terminal echo disabled.
func Password(params PasswordParams) (string, error) {
	out := params.Out
	if out == nil && params.Prompt != "" {
		out = os.Stderr
	}

	if params.Prompt != "" {
		if _, err := fmt.Fprint(out, params.Prompt); err != nil {
			return "", fmt.Errorf("writing prompt: %w", err)
		}
	}

	in := params.In
	if in == nil {
		in = os.Stdin
	}

	readPassword := params.ReadPassword
	if readPassword == nil {
		readPassword = term.ReadPassword
	}

	data, readErr := readPassword(int(in.Fd()))
	if out != nil {
		if _, promptErr := fmt.Fprintln(out); promptErr != nil && readErr == nil {
			return "", fmt.Errorf("writing prompt newline: %w", promptErr)
		}
	}
	if readErr != nil {
		return "", fmt.Errorf("reading password from terminal: %w", readErr)
	}
	return string(data), nil
}
