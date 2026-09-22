package delete

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
)

// renderParams are the inputs to the delete action renderer.
type renderParams struct {
	// Printer is the styled output layer for the safe mutation summary.
	Printer *printer.Printer
	// Result contains the removed secret identity and location.
	Result actionResult
}

// render reports the removed identity and the updated store location.
func render(p *renderParams) error {
	return p.Printer.LogMessage(fmt.Sprintf(
		"Deleted secret %q from group %q in:\n%s",
		p.Result.Key,
		p.Result.Group,
		str.QuotePath(p.Result.StorePath),
	))
}
