package termx

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ConfirmYesNoParams specifies the input, output, prompt text, and default choice
// for ConfirmYesNo.
type ConfirmYesNoParams struct {
	// Out receives the prompt string. If nil and Prompt is non-empty, defaults
	// to os.Stderr.
	Out io.Writer
	// In supplies the confirmation input. If nil, defaults to os.Stdin.
	In io.Reader
	// Prompt is written to Out before reading input. If empty, no prompt is written.
	Prompt string
	// DefaultYes determines the returned decision when the entered input is empty.
	DefaultYes bool
}

// ConfirmYesNo prompts the user with the given prompt and reads an interactive
// confirmation response. If DefaultYes is true, empty input defaults to yes;
// otherwise empty input defaults to no.
func ConfirmYesNo(params ConfirmYesNoParams) (bool, error) {
	out := params.Out
	if out == nil && params.Prompt != "" {
		out = os.Stderr
	}

	if params.Prompt != "" {
		if _, err := fmt.Fprint(out, params.Prompt); err != nil {
			return false, fmt.Errorf("writing prompt: %w", err)
		}
	}

	in := params.In
	if in == nil {
		in = os.Stdin
	}

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading response: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(response)) {
	case "":
		return params.DefaultYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, errors.New("confirmation must be y or n")
	}
}
