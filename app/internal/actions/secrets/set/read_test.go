package set

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestReaderReadsTerminalPlaintext verifies hidden confirmation, prompt routing,
// and input failures without requiring a real terminal in the test process.
func TestReaderReadsTerminalPlaintext(t *testing.T) {
	t.Parallel()

	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("Open(%q): %v", os.DevNull, err)
	}
	defer func() { _ = input.Close() }()

	for _, test := range []struct {
		name             string
		secret           string
		passwordErr      error
		noConfirm        bool
		confirmed        bool
		confirmationErr  error
		want             string
		wantErr          string
		wantPrompt       string
		wantPasswordCall int
		wantConfirmCall  int
	}{
		{
			name:             "confirmed",
			secret:           "secret",
			confirmed:        true,
			want:             "secret",
			wantPrompt:       "Secret value: \nConfirm secret of length 6? [Y/n] ",
			wantPasswordCall: 1,
			wantConfirmCall:  1,
		},
		{
			name:             "rejected",
			secret:           "secret",
			wantErr:          "secret was not confirmed",
			wantPrompt:       "Secret value: \nConfirm secret of length 6? [Y/n] ",
			wantPasswordCall: 1,
			wantConfirmCall:  1,
		},
		{
			name:             "no confirmation",
			secret:           "secret",
			noConfirm:        true,
			want:             "secret",
			wantPrompt:       "Secret value: \n",
			wantPasswordCall: 1,
		},
		{
			name:             "empty value",
			wantErr:          "no plaintext provided on terminal",
			wantPrompt:       "Secret value: \n",
			wantPasswordCall: 1,
		},
		{
			name:             "secret read failure",
			passwordErr:      errors.New("read failed"),
			wantErr:          "reading secret from terminal: read failed",
			wantPrompt:       "Secret value: \n",
			wantPasswordCall: 1,
		},
		{
			name:             "confirmation read failure",
			secret:           "secret",
			confirmationErr:  errors.New("read failed"),
			wantErr:          "read failed",
			wantPrompt:       "Secret value: \nConfirm secret of length 6? [Y/n] ",
			wantPasswordCall: 1,
			wantConfirmCall:  1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var prompt bytes.Buffer
			passwordCalls := 0
			confirmCalls := 0
			reader := newReader(readerParams{
				Stdin:  input,
				Stderr: &prompt,
				IsTerminal: func(int) bool {
					return true
				},
				NoConfirm: test.noConfirm,
				ReadPassword: func(int) ([]byte, error) {
					passwordCalls++
					return []byte(test.secret), test.passwordErr
				},
				ReadConfirmation: func(*os.File) (bool, error) {
					confirmCalls++
					return test.confirmed, test.confirmationErr
				},
			})
			got, err := reader.readSecret()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("reader.readSecret(): %v", err)
				}
				if got != test.want {
					t.Errorf("reader.readSecret() = %q, want %q", got, test.want)
				}
			} else if err == nil || err.Error() != test.wantErr {
				t.Fatalf("reader.readSecret() error = %v, want %q", err, test.wantErr)
			}
			if prompt.String() != test.wantPrompt {
				t.Errorf("prompt = %q, want %q", prompt.String(), test.wantPrompt)
			}
			if passwordCalls != test.wantPasswordCall {
				t.Errorf(
					"password reads = %d, want %d",
					passwordCalls,
					test.wantPasswordCall,
				)
			}
			if confirmCalls != test.wantConfirmCall {
				t.Errorf(
					"confirmation reads = %d, want %d",
					confirmCalls,
					test.wantConfirmCall,
				)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestParseConfirmation verifies the accepted interactive yes-or-no responses.
func TestParseConfirmation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		response string
		want     bool
		wantErr  bool
	}{
		{response: "", want: true},
		{response: "y", want: true},
		{response: "YES", want: true},
		{response: "n", want: false},
		{response: "No", want: false},
		{response: "maybe", wantErr: true},
	} {
		t.Run(test.response, func(t *testing.T) {
			got, err := parseConfirmation(test.response)
			if test.wantErr {
				if err == nil {
					t.Fatal("parseConfirmation() succeeded for invalid input")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConfirmation(): %v", err)
			}
			if got != test.want {
				t.Errorf("parseConfirmation() = %t, want %t", got, test.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestReadConfirmationInputRejectsEmptyEOF verifies end of input cannot act as
// the default yes response.
func TestReadConfirmationInputRejectsEmptyEOF(t *testing.T) {
	t.Parallel()

	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(): %v", err)
	}
	defer func() { _ = input.Close() }()
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	confirmed, err := readConfirmationInput(input)
	if confirmed {
		t.Fatal("readConfirmationInput() confirmed empty EOF")
	}
	if err == nil || err.Error() != "reading confirmation: EOF" {
		t.Fatalf("readConfirmationInput() error = %v, want EOF error", err)
	}
}

// -------------------------------------------------------------------------------------

// TestReadPlaintext verifies stdin input removes only the shell's final line
// ending and rejects an empty value.
func TestReadPlaintext(t *testing.T) {
	t.Parallel()

	got, err := readPlaintext(strings.NewReader("value\r\n"))
	if err != nil {
		t.Fatalf("readPlaintext(): %v", err)
	}
	if got != "value" {
		t.Errorf("readPlaintext() = %q, want value", got)
	}
	if _, err := readPlaintext(strings.NewReader("\n")); err == nil {
		t.Fatal("readPlaintext() accepted an empty value")
	}
}
