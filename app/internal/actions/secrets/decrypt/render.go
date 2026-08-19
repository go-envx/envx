package decrypt

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
)

// renderParams are the inputs to the decrypt action renderer.
type renderParams struct {
	// Printer is the styled output layer for the summary and skipped-group warnings.
	Printer *printer.Printer
	// Result contains the changed identities, skipped groups, and store location.
	Result actionResult
	// Verbose lists each changed identity in addition to the summary count.
	Verbose bool
}

// render reports any skipped-group warnings on stderr first, then the decrypted
// identities on stdout, never a secret value. Leading with the warnings surfaces
// the attention-worthy signal before the routine summary; a blank line separates
// the two when both are present and share a terminal.
func render(p *renderParams) error {
	if err := renderWarnings(p.Printer, p.Result.Unavailable); err != nil {
		return err
	}
	if len(p.Result.Unavailable) > 0 && len(p.Result.Changed) > 0 {
		if err := p.Printer.LogBlank(); err != nil {
			return err
		}
	}
	return renderSummary(p.Printer, p.Result, p.Verbose)
}

// renderSummary reports how many values were decrypted and where, listing each
// changed identity only when verbose. It stays silent when nothing was decrypted
// but a group was skipped, since the stderr warning already explains the outcome.
func renderSummary(p *printer.Printer, result actionResult, verbose bool) error {
	if len(result.Changed) == 0 {
		if len(result.Unavailable) > 0 {
			return nil
		}
		return p.LogMessage("No encrypted values to decrypt.")
	}

	var b strings.Builder
	fmt.Fprintf(
		&b, "Decrypted %s in:\n%s",
		str.Pluralize(len(result.Changed), "secret", "secrets"),
		str.QuotePath(result.StorePath),
	)
	if verbose {
		for _, secret := range result.Changed {
			fmt.Fprintf(&b, "\n  %s/%s", secret.Group, secret.Key)
		}
	}
	return p.LogMessage(b.String())
}

// renderWarnings reports each group skipped because no private key was available.
func renderWarnings(p *printer.Printer, groups []string) error {
	for _, group := range groups {
		if err := p.LogWarning(fmt.Sprintf(
			"no private key available for group %q; its secrets were left encrypted",
			group,
		)); err != nil {
			return err
		}
	}
	return nil
}
