package decrypt

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ANSI codes used to highlight a warning when standard error is a terminal.
const (
	ansiYellow = "\033[33m"
	ansiReset  = "\033[0m"
)

// renderParams are the inputs to the decrypt action renderer.
type renderParams struct {
	// Writer receives the safe mutation summary on standard output.
	Writer io.Writer
	// ErrWriter receives skipped-group warnings on standard error.
	ErrWriter io.Writer
	// Result contains the changed identities, skipped groups, and store location.
	Result actionResult
	// Verbose lists each changed identity in addition to the summary count.
	Verbose bool
	// Color highlights warnings when standard error is a terminal.
	Color bool
}

// render reports the decrypted identities on stdout and any skipped-group
// warnings on stderr, never a secret value.
func render(p *renderParams) error {
	if err := renderSummary(p.Writer, p.Result, p.Verbose); err != nil {
		return err
	}
	return renderWarnings(p.ErrWriter, p.Result.Unavailable, p.Color)
}

// renderSummary reports how many values were decrypted and where, listing each
// changed identity only when verbose. It stays silent when nothing was decrypted
// but a group was skipped, since the stderr warning already explains the outcome.
func renderSummary(w io.Writer, result actionResult, verbose bool) error {
	if len(result.Changed) == 0 {
		if len(result.Unavailable) > 0 {
			return nil
		}
		_, err := fmt.Fprintln(w, "No encrypted values to decrypt.")
		return err
	}

	var b strings.Builder
	fmt.Fprintf(
		&b, "Decrypted %d secret(s) in:\n%s\n",
		len(result.Changed), renderStorePath(result.StorePath),
	)
	if verbose {
		for _, secret := range result.Changed {
			fmt.Fprintf(&b, "  %s/%s\n", secret.Group, secret.Key)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// renderWarnings reports each group skipped because no private key was available.
func renderWarnings(w io.Writer, groups []string, color bool) error {
	label := "warning:"
	if color {
		label = ansiYellow + label + ansiReset
	}
	for _, group := range groups {
		if _, err := fmt.Fprintf(
			w, "%s no private key available for group %q; its secrets were left encrypted\n",
			label, group,
		); err != nil {
			return err
		}
	}
	return nil
}

// renderStorePath quotes a path only when a space would make its boundary
// ambiguous in terminal output.
func renderStorePath(path string) string {
	if strings.Contains(path, " ") {
		return strconv.Quote(path)
	}
	return path
}
