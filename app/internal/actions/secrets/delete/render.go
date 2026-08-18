package delete

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-envx/envx/app/internal/printer"
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
		renderStorePath(p.Result.StorePath),
	))
}

// renderStorePath quotes a path only when a space would make its boundary
// ambiguous in terminal output.
func renderStorePath(path string) string {
	if strings.Contains(path, " ") {
		return strconv.Quote(path)
	}
	return path
}
