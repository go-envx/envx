package set

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// -------------------------------------------------------------------------------------

// readerParams supplies the input, output, and terminal dependencies for a
// secret reader.
type readerParams struct {
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

// -------------------------------------------------------------------------------------

// reader owns secret input selection and interactive terminal reading.
type reader struct {
	// params holds the reader's input and terminal dependencies.
	params readerParams
}

// -------------------------------------------------------------------------------------

// newReader constructs a secret reader and fills in the production terminal
// dependencies when tests have not supplied replacements.
func newReader(params readerParams) *reader {
	if params.IsTerminal == nil {
		params.IsTerminal = term.IsTerminal
	}
	if params.ReadPassword == nil {
		params.ReadPassword = term.ReadPassword
	}
	if params.ReadConfirmation == nil {
		params.ReadConfirmation = readConfirmationInput
	}
	return &reader{params: params}
}

// -------------------------------------------------------------------------------------

// readSecret reads piped plaintext or confirmed hidden terminal input.
func (r *reader) readSecret() (string, error) {
	file, ok := r.params.Stdin.(*os.File)
	if !ok || !r.params.IsTerminal(int(file.Fd())) {
		return readPlaintext(r.params.Stdin)
	}
	return r.readTerminalPlaintext(file)
}

// -------------------------------------------------------------------------------------

// readPlaintext reads a piped value and removes one line ending supplied by the
// shell while preserving all other plaintext bytes.
func readPlaintext(input io.Reader) (string, error) {
	data, err := io.ReadAll(input)
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

// -------------------------------------------------------------------------------------

// readTerminalPlaintext reads one hidden value and asks for confirmation before
// returning it.
func (r *reader) readTerminalPlaintext(input *os.File) (string, error) {
	secret, err := r.readTerminalValue(input, "Secret value")
	if err != nil {
		return "", err
	}
	if secret == "" {
		return "", errors.New("no plaintext provided on terminal")
	}
	if r.params.NoConfirm {
		return secret, nil
	}
	confirmationPrompt := fmt.Sprintf(
		"Confirm secret of length %d? [Y/n] ",
		utf8.RuneCountInString(secret),
	)
	if _, err := fmt.Fprint(r.params.Stderr, confirmationPrompt); err != nil {
		return "", fmt.Errorf("writing confirmation prompt: %w", err)
	}
	confirmed, err := r.params.ReadConfirmation(input)
	if err != nil {
		return "", err
	}
	if !confirmed {
		return "", errors.New("secret was not confirmed")
	}
	return secret, nil
}

// -------------------------------------------------------------------------------------

// readTerminalValue writes one prompt and reads one hidden terminal value.
func (r *reader) readTerminalValue(input *os.File, label string) (string, error) {
	if _, err := fmt.Fprintf(r.params.Stderr, "%s: ", label); err != nil {
		return "", fmt.Errorf("writing secret prompt: %w", err)
	}
	data, readErr := r.params.ReadPassword(int(input.Fd()))
	if _, promptErr := fmt.Fprintln(r.params.Stderr); promptErr != nil && readErr == nil {
		return "", fmt.Errorf("writing secret prompt: %w", promptErr)
	}
	if readErr != nil {
		return "", fmt.Errorf("reading secret from terminal: %w", readErr)
	}
	return string(data), nil
}

// -------------------------------------------------------------------------------------

// readConfirmationInput reads and parses the interactive yes-or-no response.
func readConfirmationInput(input *os.File) (bool, error) {
	response, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading confirmation: %w", err)
	}
	if errors.Is(err, io.EOF) && strings.TrimSpace(response) == "" {
		return false, fmt.Errorf("reading confirmation: %w", err)
	}
	return parseConfirmation(response)
}

// -------------------------------------------------------------------------------------

// parseConfirmation maps an interactive response to a confirmation decision.
func parseConfirmation(response string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "", "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, errors.New("confirmation must be y or n")
	}
}
