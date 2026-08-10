package print

import (
	"fmt"
	"io"
	"strings"
)

// renderParams bundles the output sink and structured keypair result.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the ephemeral keypair to render.
	Result actionResult
}

// render writes both halves of the ephemeral keypair to p.Writer.
func render(p *renderParams) error {
	_, err := fmt.Fprintf(
		p.Writer,
		"%s\n%s\n",
		formatKey(p.Result.Keypair.PublicKey),
		formatKey(p.Result.Keypair.PrivateKey),
	)
	return err
}

// formatKey adds display spacing after the key type's first colon while keeping
// the cipher-provided key label and material unchanged.
func formatKey(key string) string {
	prefix, value, found := strings.Cut(key, ":")
	if !found {
		return key
	}
	return prefix + ": " + value
}
