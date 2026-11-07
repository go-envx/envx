package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var readPassword = term.ReadPassword

// Password prompts for and reads a secret value without echoing input
// on interactive terminals. If in is an active terminal, terminal echo is
// disabled while reading. If in is not a terminal (e.g. piped input),
// the line is read directly without echo modification. An optional prompt
// is written to out when prompt is non-empty and out is non-nil.
func Password(in io.Reader, out io.Writer, prompt string) (string, error) {
	if prompt != "" && out != nil {
		if _, err := fmt.Fprint(out, prompt); err != nil {
			return "", fmt.Errorf("writing prompt: %w", err)
		}
	}

	if in == nil {
		return "", io.EOF
	}

	var file *os.File
	if f, ok := in.(*os.File); ok {
		file = f
	}

	if file != nil && IsTerminal(file) {
		data, readErr := readPassword(int(file.Fd()))
		if out != nil {
			if _, nlErr := fmt.Fprintln(out); nlErr != nil && readErr == nil {
				return "", fmt.Errorf("writing prompt newline: %w", nlErr)
			}
		}
		if readErr != nil {
			return "", fmt.Errorf("reading password from terminal: %w", readErr)
		}
		return string(data), nil
	}

	reader := bufio.NewReader(in)
	line, readErr := reader.ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", fmt.Errorf("reading password: %w", readErr)
	}

	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")

	if readErr != nil && line == "" {
		return "", readErr
	}

	if out != nil && prompt != "" {
		if _, nlErr := fmt.Fprintln(out); nlErr != nil {
			return "", fmt.Errorf("writing prompt newline: %w", nlErr)
		}
	}

	return line, nil
}
