package set

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/go-envx/envx/app/internal/config"
)

// -------------------------------------------------------------------------------------

// actionParams identifies the one secret being written.
type actionParams struct {
	// Group identifies the case-insensitive key group.
	Group string
	// Key identifies the case-sensitive secret entry.
	Key string
}

// -------------------------------------------------------------------------------------

// streams bundles the input and diagnostic sinks used by the set action.
type streams struct {
	// Stdin supplies plaintext from a pipe or terminal.
	Stdin io.Reader
	// Stderr receives the hidden-input prompt and diagnostics.
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------

// actionResult carries only safe mutation metadata to the renderer.
type actionResult struct {
	// Group is the normalized key group.
	Group string
	// Key is the stored secret entry name.
	Key string
	// StorePath is the workspace store that received the ciphertext.
	StorePath string
}

// -------------------------------------------------------------------------------------

// execute reads one explicit secret value and delegates encryption and storage
// to the root secrets manager.
func execute(
	p actionParams,
	in *config.Input,
	s streams,
) (actionResult, error) {
	// Resolve the workspace configuration and encryption settings.
	c, err := config.ResolveWorkspace(in)
	if err != nil {
		return actionResult{}, err
	}
	// Create the manager responsible for encrypted secret storage.
	manager, err := config.NewSecretsManager(c.Secrets, c.Cipher)
	if err != nil {
		return actionResult{}, err
	}
	// Function to read secret via terminal input.
	readSecret := func() (string, error) {
		return readInput(s.Stdin, s.Stderr)
	}
	// Set the secret value via the secrets manager.
	if err := manager.Set(p.Group, p.Key, readSecret); err != nil {
		return actionResult{}, err
	}
	// Return secret metadata provided by secrets manager.
	return actionResult{
		Group:     strings.ToLower(p.Group),
		Key:       p.Key,
		StorePath: c.Secrets.SecretsPath,
	}, nil
}

// -------------------------------------------------------------------------------------

// readInput selects no-echo terminal input only for an actual terminal; all
// other readers are treated as explicit stdin and consumed as a stream.
func readInput(input io.Reader, prompt io.Writer) (string, error) {
	file, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return readPlaintext(input)
	}
	return readTerminalPlaintext(file, prompt)
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

// readTerminalPlaintext prompts on stderr and reads one line with terminal echo
// disabled so the value never appears on the screen.
func readTerminalPlaintext(input *os.File, prompt io.Writer) (string, error) {
	if _, err := fmt.Fprint(prompt, "Secret value: "); err != nil {
		return "", fmt.Errorf("writing secret prompt: %w", err)
	}
	data, err := term.ReadPassword(int(input.Fd()))
	if _, promptErr := fmt.Fprintln(prompt); err == nil && promptErr != nil {
		return "", fmt.Errorf("writing secret prompt: %w", promptErr)
	}
	if err != nil {
		return "", fmt.Errorf("reading secret from terminal: %w", err)
	}
	if len(data) == 0 {
		return "", errors.New("no plaintext provided on terminal")
	}
	return string(data), nil
}
