package set

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
)

// renderParams are the inputs to the set action renderer.
type renderParams struct {
	// Printer is the styled output layer for the write confirmation.
	Printer *printer.Printer
	// Result contains the written key and overlay location.
	Result actionResult
}

// render confirms the written key and the overlay file it landed in.
func render(p *renderParams) error {
	return p.Printer.LogMessage(fmt.Sprintf(
		"Set %q in:\n%s",
		p.Result.Key,
		str.QuotePath(p.Result.OverlayPath),
	))
}
