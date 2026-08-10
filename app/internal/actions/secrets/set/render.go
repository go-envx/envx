package set

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// renderParams are the inputs to the set action renderer.
type renderParams struct {
	// Writer receives the safe mutation summary.
	Writer io.Writer
	// Result contains the stored secret identity and location.
	Result actionResult
}

// render reports the stored identity without printing plaintext or ciphertext.
func render(p *renderParams) error {
	_, err := fmt.Fprintf(
		p.Writer,
		"Stored secret %q in group %q at:\n%s\n",
		p.Result.Key,
		p.Result.Group,
		renderStorePath(p.Result.StorePath),
	)
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
