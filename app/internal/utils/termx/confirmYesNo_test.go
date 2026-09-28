package termx_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/utils/termx"
)

func TestConfirmYesNo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		prompt     string
		defaultYes bool
		want       bool
		wantErr    bool
		wantPrompt string
	}{
		{
			name:       "empty input with default yes",
			input:      "\n",
			prompt:     "Proceed? [Y/n] ",
			defaultYes: true,
			want:       true,
			wantPrompt: "Proceed? [Y/n] ",
		},
		{
			name:       "empty input with default no",
			input:      "\n",
			defaultYes: false,
			want:       false,
		},
		{
			name:       "yes variations",
			input:      "y\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "uppercase YES",
			input:      "YES\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "no variations",
			input:      "n\n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "uppercase NO",
			input:      "NO\n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "invalid input",
			input:      "maybe\n",
			defaultYes: true,
			wantErr:    true,
		},
		{
			name:       "empty input string with EOF",
			input:      "",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "empty input string with EOF and default no",
			input:      "",
			defaultYes: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			in := strings.NewReader(tt.input)

			got, err := termx.ConfirmYesNo(termx.ConfirmYesNoParams{
				Out:        &out,
				In:         in,
				Prompt:     tt.prompt,
				DefaultYes: tt.defaultYes,
			})
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
}

type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write failure")
}

func TestConfirmYesNoErrorWritingPrompt(t *testing.T) {
	errWriter := &failingWriter{}
	_, err := termx.ConfirmYesNo(termx.ConfirmYesNoParams{
		Out:        errWriter,
		In:         strings.NewReader("y\n"),
		Prompt:     "Prompt: ",
		DefaultYes: true,
	})
	if err == nil {
		t.Error("ConfirmYesNo() expected error when writing prompt fails, got nil")
	}
}
