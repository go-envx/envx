package encrypt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
)

// renderParams are the inputs to the encrypt action renderer.
type renderParams struct {
	// Printer is the styled output layer for the safe mutation summary.
	Printer *printer.Printer
	// Result contains the changed identities and store location.
	Result actionResult
	// Verbose lists each changed identity in addition to the summary count.
	Verbose bool
}

// render reports how many values were encrypted and where, never a secret value.
// It lists each changed identity only when Verbose is set, and reports plainly
// when nothing needed encrypting.
func render(p *renderParams) error {
	if len(p.Result.Changed) == 0 {
		return p.Printer.LogMessage("No plaintext values to encrypt.")
	}

	var b strings.Builder
	fmt.Fprintf(
		&b, "Encrypted %s in:\n%s",
		str.Pluralize(len(p.Result.Changed), "secret", "secrets"),
		renderStorePath(p.Result.StorePath),
	)
	if p.Verbose {
		for _, secret := range p.Result.Changed {
			fmt.Fprintf(&b, "\n  %s/%s", secret.Group, secret.Key)
		}
	}
	return p.Printer.LogMessage(b.String())
}

// renderStorePath quotes a path only when a space would make its boundary
// ambiguous in terminal output.
func renderStorePath(path string) string {
	if strings.Contains(path, " ") {
		return strconv.Quote(path)
	}
	return path
}
