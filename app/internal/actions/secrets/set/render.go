package set

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
)

// renderParams are the inputs to the set action renderer.
type renderParams struct {
	// Printer is the styled output layer for the safe mutation summary.
	Printer *printer.Printer
	// Result contains the stored secret identity and location.
	Result actionResult
}

// render reports the stored identity without printing plaintext or ciphertext.
func render(p *renderParams) error {
	return p.Printer.LogMessage(fmt.Sprintf(
		"Stored secret %q in group %q at:\n%s",
		p.Result.Key,
		p.Result.Group,
		str.QuotePath(p.Result.StorePath),
	))
}
