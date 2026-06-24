package get

import (
	"fmt"
	"io"
)

// -------------------------------------------------------------------------------------

// renderParams bundles everything render needs: the output sink and the
// structured result.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the structured data to render.
	Result actionResult
}

// -------------------------------------------------------------------------------------

// render writes the resolved value to p.Writer.
func render(p *renderParams) error {
	_, err := fmt.Fprintln(p.Writer, p.Result.Value)
	return err
}
