package print

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/printer"
)

// renderParams bundles the output sink and structured keypair result.
type renderParams struct {
	// Printer is the styled output layer for the ephemeral keypair.
	Printer *printer.Printer
	// Result is the ephemeral keypair to render.
	Result actionResult
}

// render writes both halves of the ephemeral keypair through the printer.
func render(p *renderParams) error {
	return p.Printer.LogMessage(fmt.Sprintf(
		"%s\n%s",
		formatKey(p.Result.Keypair.PublicKey),
		formatKey(p.Result.Keypair.PrivateKey),
	))
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
