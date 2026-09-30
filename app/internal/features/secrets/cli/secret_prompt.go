package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/go-envx/envx/app/internal/utils/termx"
)

// secretPromptParams supplies the input, output, and terminal dependencies for
// secret input prompting.
type secretPromptParams struct {
	// Stdin supplies plaintext from a pipe or terminal.
	Stdin io.Reader
	// Stderr receives the hidden-input prompt and diagnostics.
	Stderr io.Writer
	// IsTerminal identifies terminal file descriptors for input selection.
	IsTerminal func(int) bool
	// ReadPassword reads one value from a terminal with echo disabled.
	ReadPassword func(int) ([]byte, error)
	// ReadConfirmation reads the interactive yes-or-no confirmation.
	ReadConfirmation func(*os.File) (bool, error)
	// NoConfirm skips the interactive confirmation after hidden input.
	NoConfirm bool
}

// secretPrompt owns secret input selection and interactive terminal reading.
type secretPrompt struct {
	// params holds the prompt's input and terminal dependencies.
	params secretPromptParams
	// terminal holds the verified terminal file descriptor, or nil if input is piped.
	terminal *os.File
}

// newSecretPrompt constructs a secret prompt and detects interactive terminal input.
func newSecretPrompt(params secretPromptParams) *secretPrompt {
	sp := &secretPrompt{params: params}
	if file, ok := params.Stdin.(*os.File); ok {
		isTerm := termx.IsTerminal(file)
		if params.IsTerminal != nil {
			isTerm = params.IsTerminal(int(file.Fd()))
		}
		if isTerm {
			sp.terminal = file
		}
	}
	return sp
}

// readSecret reads piped plaintext or confirmed hidden terminal input.
func (p *secretPrompt) readSecret() (string, error) {
	if p.terminal == nil {
		return p.readPlaintext()
	}
	return p.readTerminalPlaintext()
}

// readPlaintext reads a piped value and removes one line ending supplied by the
// shell while preserving all other plaintext bytes.
func (p *secretPrompt) readPlaintext() (string, error) {
	data, err := io.ReadAll(p.params.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading plaintext from stdin: %w", err)
	}
	plaintext := strings.TrimSuffix(string(data), "\n")
	plaintext = strings.TrimSuffix(plaintext, "\r")
	if plaintext == "" {
		return "", errors.New("no plaintext provided on stdin")
	}
	return plaintext, nil
}

// readTerminalPlaintext reads one hidden value and asks for confirmation before
// returning it.
func (p *secretPrompt) readTerminalPlaintext() (string, error) {
	secret, err := termx.Password(termx.PasswordParams{
		Out:          p.params.Stderr,
		In:           p.terminal,
		Prompt:       "Secret value: ",
		ReadPassword: p.params.ReadPassword,
	})
	if err != nil {
		return "", err
	}
	if secret == "" {
		return "", errors.New("no plaintext provided on terminal")
	}
	confirmed, err := p.confirmSecret(secret)
	if err != nil {
		return "", err
	}
	if !confirmed {
		return "", errors.New("secret was not confirmed")
	}
	return secret, nil
}

// confirmSecret prompts the user to confirm the length of the entered secret
// unless confirmation was explicitly disabled.
func (p *secretPrompt) confirmSecret(secret string) (bool, error) {
	if p.params.NoConfirm {
		return true, nil
	}
	if p.params.ReadConfirmation != nil {
		confirmationPrompt := fmt.Sprintf(
			"Confirm secret of length %d? [Y/n] ",
			utf8.RuneCountInString(secret),
		)
		if _, err := fmt.Fprint(p.params.Stderr, confirmationPrompt); err != nil {
			return false, fmt.Errorf("writing confirmation prompt: %w", err)
		}
		return p.params.ReadConfirmation(p.terminal)
	}
	return termx.ConfirmYesNo(termx.ConfirmYesNoParams{
		Out: p.params.Stderr,
		In:  p.terminal,
		Prompt: fmt.Sprintf(
			"Confirm secret of length %d? [Y/n] ",
			utf8.RuneCountInString(secret),
		),
		DefaultYes: true,
	})
}
