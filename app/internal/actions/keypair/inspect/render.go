package inspect

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/internal/secrets"
)

// renderParams bundles the output sink and keypair inspection result.
type renderParams struct {
	// Printer is the styled output layer for the inspection summary.
	Printer *printer.Printer
	// Result is the public key metadata and private-key status to render.
	Result secrets.KeypairMetadata
}

// render prints public metadata and private-key status only.
func render(p *renderParams) error {
	return p.Printer.LogMessage(fmt.Sprintf(
		"Keypair for group %q:\n"+
			"  public key: %s\n"+
			"  private key status: %s",
		p.Result.Group,
		p.Result.PublicKey,
		p.Result.PrivateKeyStatus,
	))
}
