package get

import (
	"fmt"
	"io"
)

// renderParams are the inputs to the get action renderer.
type renderParams struct {
	// Writer receives the decrypted plaintext.
	Writer io.Writer
	// Result contains the decrypted secret value.
	Result actionResult
}

// render prints the decrypted plaintext followed by a newline so the value is
// convenient to pipe.
func render(p *renderParams) error {
	_, err := fmt.Fprintln(p.Writer, p.Result.Value)
	return err
}
