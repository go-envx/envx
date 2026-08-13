package encrypt

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// renderParams are the inputs to the encrypt action renderer.
type renderParams struct {
	// Writer receives the safe mutation summary.
	Writer io.Writer
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
		_, err := fmt.Fprintln(p.Writer, "No plaintext values to encrypt.")
		return err
	}

	var b strings.Builder
	fmt.Fprintf(
		&b, "Encrypted %d secret(s) in:\n%s\n",
		len(p.Result.Changed), renderStorePath(p.Result.StorePath),
	)
	if p.Verbose {
		for _, secret := range p.Result.Changed {
			fmt.Fprintf(&b, "  %s/%s\n", secret.Group, secret.Key)
		}
	}
	_, err := io.WriteString(p.Writer, b.String())
	return err
}

// renderStorePath quotes a path only when a space would make its boundary
// ambiguous in terminal output.
func renderStorePath(path string) string {
	if strings.Contains(path, " ") {
		return strconv.Quote(path)
	}
	return path
}
