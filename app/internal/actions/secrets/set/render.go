package set

import (
	"fmt"
	"io"
)

// -------------------------------------------------------------------------------------

// renderParams are the inputs to the set action renderer.
type renderParams struct {
	// Writer receives the safe mutation summary.
	Writer io.Writer
	// Result contains the stored secret identity and location.
	Result actionResult
}

// -------------------------------------------------------------------------------------

// render reports the stored identity without printing plaintext or ciphertext.
func render(p *renderParams) error {
	_, err := fmt.Fprintf(
		p.Writer,
		"Stored secret %q in group %q:\n  secrets store: %s\n",
		p.Result.Key,
		p.Result.Group,
		p.Result.StorePath,
	)
	return err
}
