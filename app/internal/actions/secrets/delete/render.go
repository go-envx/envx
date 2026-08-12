package delete

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// renderParams are the inputs to the delete action renderer.
type renderParams struct {
	// Writer receives the safe mutation summary.
	Writer io.Writer
	// Result contains the removed secret identity and location.
	Result actionResult
}

// render reports the removed identity and the updated store location.
func render(p *renderParams) error {
	_, err := fmt.Fprintf(
		p.Writer,
		"Deleted secret %q from group %q in:\n%s\n",
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
