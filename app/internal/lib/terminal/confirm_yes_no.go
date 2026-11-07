package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ConfirmYesNo prompts the user with prompt on out and reads an interactive
// confirmation response from in. If prompt contains "[Y/n]", empty input
// defaults to true (yes). Otherwise, empty input defaults to false (no).
// Valid affirmative responses are "y" and "yes" (case-insensitive).
// Valid negative responses are "n" and "no" (case-insensitive).
// Any other response returns an error indicating confirmation must be y or n.
func ConfirmYesNo(in io.Reader, out io.Writer, prompt string) (bool, error) {
	if prompt != "" && out != nil {
		if _, err := fmt.Fprint(out, prompt); err != nil {
			return false, fmt.Errorf("writing prompt: %w", err)
		}
	}

	if in == nil {
		in = strings.NewReader("")
	}

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading response: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(response)) {
	case "":
		return strings.Contains(prompt, "[Y/n]"), nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, errors.New("confirmation must be y or n")
	}
}
