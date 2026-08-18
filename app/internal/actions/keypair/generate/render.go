package generate

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/printer"
)

// renderParams bundles the output sink and safe generation result.
type renderParams struct {
	// Printer is the styled output layer for the safe generation summary.
	Printer *printer.Printer
	// Result is the safe generated keypair metadata to render.
	Result actionResult
}

// render prints safe generation metadata without private-key material.
func render(p *renderParams) error {
	return p.Printer.LogMessage(fmt.Sprintf(
		"Generated keypair for group %q:\n"+
			"  public key: %s\n"+
			"  secrets store: %s\n"+
			"  private key file: %s",
		p.Result.Metadata.Group,
		p.Result.Metadata.PublicKey,
		p.Result.SecretsPath,
		p.Result.KeysPath,
	))
}
