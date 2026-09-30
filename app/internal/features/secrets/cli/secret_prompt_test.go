package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pipe supplies test bytes as an os.File so IsTerminal is evaluated on a real
// descriptor without relying on an interactive terminal.
func pipe(t *testing.T, content string) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(): %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
	})
	go func() {
		defer func() { _ = w.Close() }()
		_, _ = w.WriteString(content)
	}()
	return r
}

// terminalFile creates an open file with a valid file descriptor to simulate
// terminal input when IsTerminal is forced to return true.
func terminalFile(t *testing.T) *os.File {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "terminal"))
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	t.Cleanup(func() {
		_ = file.Close()
	})
	return file
}

// TestReadSecretFromPipedStdin verifies piped input is read without prompting
// and that trailing newlines are stripped.
func TestReadSecretFromPipedStdin(t *testing.T) {
	t.Parallel()

	r := newSecretPrompt(secretPromptParams{
		Stdin:      pipe(t, "super-secret\n"),
		Stderr:     new(bytes.Buffer),
		IsTerminal: func(int) bool { return false },
	})
	got, err := r.readSecret()
	if err != nil {
		t.Fatalf("readSecret(): %v", err)
	}
	if got != "super-secret" {
		t.Errorf("readSecret() = %q, want %q", got, "super-secret")
	}
}

// TestReadSecretPreservesInternalNewlines verifies only the trailing newline is
// stripped from piped input.
func TestReadSecretPreservesInternalNewlines(t *testing.T) {
	t.Parallel()

	const multiline = "line1\nline2\nline3"
	r := newSecretPrompt(secretPromptParams{
		Stdin:      pipe(t, multiline+"\n"),
		Stderr:     new(bytes.Buffer),
		IsTerminal: func(int) bool { return false },
	})
	got, err := r.readSecret()
	if err != nil {
		t.Fatalf("readSecret(): %v", err)
	}
	if got != multiline {
		t.Errorf("readSecret() = %q, want %q", got, multiline)
	}
}

// TestReadSecretRejectsEmptyPipedStdin verifies empty piped input is an error.
func TestReadSecretRejectsEmptyPipedStdin(t *testing.T) {
	t.Parallel()

	r := newSecretPrompt(secretPromptParams{
		Stdin:      pipe(t, "\n"),
		Stderr:     new(bytes.Buffer),
		IsTerminal: func(int) bool { return false },
	})
	if _, err := r.readSecret(); err == nil {
		t.Fatal("readSecret() accepted an empty piped value")
	}
}

// TestReadSecretFromTerminalWithConfirmation verifies the interactive flow
// prompts for the value, confirms the length, and returns the entered plaintext.
func TestReadSecretFromTerminalWithConfirmation(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	file := terminalFile(t)
	r := newSecretPrompt(secretPromptParams{
		Stdin:            file,
		Stderr:           &stderr,
		IsTerminal:       func(int) bool { return true },
		ReadPassword:     func(int) ([]byte, error) { return []byte("secret-value"), nil },
		ReadConfirmation: func(*os.File) (bool, error) { return true, nil },
	})

	got, err := r.readSecret()
	if err != nil {
		t.Fatalf("readSecret(): %v", err)
	}
	if got != "secret-value" {
		t.Errorf("readSecret() = %q, want %q", got, "secret-value")
	}

	prompt := stderr.String()
	if !strings.Contains(prompt, "Secret value: ") {
		t.Errorf("prompt = %q, want Secret value: ", prompt)
	}
	if !strings.Contains(prompt, "Confirm secret of length 12? [Y/n] ") {
		t.Errorf("prompt = %q, want length confirmation", prompt)
	}
}

// TestReadSecretFromTerminalRejectedConfirmation verifies unconfirmed terminal
// input is rejected.
func TestReadSecretFromTerminalRejectedConfirmation(t *testing.T) {
	t.Parallel()

	r := newSecretPrompt(secretPromptParams{
		Stdin:            terminalFile(t),
		Stderr:           new(bytes.Buffer),
		IsTerminal:       func(int) bool { return true },
		ReadPassword:     func(int) ([]byte, error) { return []byte("secret-value"), nil },
		ReadConfirmation: func(*os.File) (bool, error) { return false, nil },
	})

	if _, err := r.readSecret(); err == nil {
		t.Fatal("readSecret() accepted an unconfirmed secret")
	}
}

// TestReadSecretFromTerminalSkipsConfirmationWhenRequested verifies NoConfirm
// returns the value without prompting for confirmation.
func TestReadSecretFromTerminalSkipsConfirmationWhenRequested(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	confirmationCalled := false
	r := newSecretPrompt(secretPromptParams{
		Stdin:        terminalFile(t),
		Stderr:       &stderr,
		IsTerminal:   func(int) bool { return true },
		ReadPassword: func(int) ([]byte, error) { return []byte("secret-value"), nil },
		ReadConfirmation: func(*os.File) (bool, error) {
			confirmationCalled = true
			return true, nil
		},
		NoConfirm: true,
	})

	got, err := r.readSecret()
	if err != nil {
		t.Fatalf("readSecret(): %v", err)
	}
	if got != "secret-value" {
		t.Errorf("readSecret() = %q, want %q", got, "secret-value")
	}
	if confirmationCalled {
		t.Error("readSecret() requested confirmation when NoConfirm was set")
	}
	if strings.Contains(stderr.String(), "Confirm secret") {
		t.Errorf("prompt = %q, want no confirmation prompt", stderr.String())
	}
}
