package terminal_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/terminal"
)

func TestConfirmYesNo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		prompt     string
		want       bool
		wantErr    bool
		wantPrompt string
	}{
		{
			name:       "empty input with [Y/n] default yes",
			input:      "\n",
			prompt:     "Proceed? [Y/n] ",
			want:       true,
			wantPrompt: "Proceed? [Y/n] ",
		},
		{
			name:   "empty input with [y/N] default no",
			input:  "\n",
			prompt: "Proceed? [y/N] ",
			want:   false,
		},
		{
			name:   "empty input without indicator defaults to no",
			input:  "\n",
			prompt: "Proceed? ",
			want:   false,
		},
		{
			name:   "yes lowercase",
			input:  "y\n",
			prompt: "Proceed? [y/N] ",
			want:   true,
		},
		{
			name:   "yes word lowercase",
			input:  "yes\n",
			prompt: "Proceed? [y/N] ",
			want:   true,
		},
		{
			name:   "yes uppercase",
			input:  "YES\n",
			prompt: "Proceed? [y/N] ",
			want:   true,
		},
		{
			name:   "no lowercase",
			input:  "n\n",
			prompt: "Proceed? [Y/n] ",
			want:   false,
		},
		{
			name:   "no word lowercase",
			input:  "no\n",
			prompt: "Proceed? [Y/n] ",
			want:   false,
		},
		{
			name:   "no uppercase",
			input:  "NO\n",
			prompt: "Proceed? [Y/n] ",
			want:   false,
		},
		{
			name:    "invalid input",
			input:   "maybe\n",
			prompt:  "Proceed? [Y/n] ",
			wantErr: true,
		},
		{
			name:   "EOF with [Y/n] default yes",
			input:  "",
			prompt: "Proceed? [Y/n] ",
			want:   true,
		},
		{
			name:   "EOF with [y/N] default no",
			input:  "",
			prompt: "Proceed? [y/N] ",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			in := strings.NewReader(tt.input)

			got, err := terminal.ConfirmYesNo(in, &out, tt.prompt)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ConfirmYesNo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("ConfirmYesNo() = %v, want %v", got, tt.want)
			}
			if tt.wantPrompt != "" && out.String() != tt.wantPrompt {
				t.Errorf("prompt = %q, want %q", out.String(), tt.wantPrompt)
			}
		})
	}

	t.Run("nil input defaults according to prompt", func(t *testing.T) {
		var out bytes.Buffer
		got, err := terminal.ConfirmYesNo(nil, &out, "Proceed? [Y/n] ")
		if err != nil {
			t.Fatalf("ConfirmYesNo() error = %v", err)
		}
		if !got {
			t.Errorf("ConfirmYesNo() = %v, want true", got)
		}

		got, err = terminal.ConfirmYesNo(nil, &out, "Proceed? [y/N] ")
		if err != nil {
			t.Fatalf("ConfirmYesNo() error = %v", err)
		}
		if got {
			t.Errorf("ConfirmYesNo() = %v, want false", got)
		}
	})

	t.Run("nil output with valid input", func(t *testing.T) {
		got, err := terminal.ConfirmYesNo(strings.NewReader("y\n"), nil, "Proceed? [Y/n] ")
		if err != nil {
			t.Fatalf("ConfirmYesNo() error = %v", err)
		}
		if !got {
			t.Errorf("ConfirmYesNo() = %v, want true", got)
		}
	})

	t.Run("error writing prompt", func(t *testing.T) {
		_, err := terminal.ConfirmYesNo(strings.NewReader("y\n"), failingWriter{}, "Prompt: ")
		if err == nil {
			t.Fatal("ConfirmYesNo() expected error when prompt write fails, got nil")
		}
		if !strings.Contains(err.Error(), "writing prompt") {
			t.Errorf("error = %v, want writing prompt error", err)
		}
	})

	t.Run("error reading from stream", func(t *testing.T) {
		var out bytes.Buffer
		_, err := terminal.ConfirmYesNo(failingReader{}, &out, "Prompt: ")
		if err == nil {
			t.Fatal("ConfirmYesNo() expected error when stream read fails, got nil")
		}
		if !strings.Contains(err.Error(), "reading response") {
			t.Errorf("error = %v, want reading response error", err)
		}
	})
}
