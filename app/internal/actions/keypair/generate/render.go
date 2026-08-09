package generate

import (
	"fmt"
	"io"
)

// -------------------------------------------------------------------------------------

// renderParams bundles the output sink and safe generation result.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the safe generated keypair metadata to render.
	Result actionResult
}

// -------------------------------------------------------------------------------------

// render prints safe generation metadata without private-key material.
func render(p *renderParams) error {
	_, err := fmt.Fprintf(
		p.Writer,
		"Generated keypair for group %q:\n"+
			"  public key: %s\n"+
			"  secrets store: %s\n"+
			"  private key file: %s\n",
		p.Result.Metadata.Group,
		p.Result.Metadata.PublicKey,
		p.Result.SecretsPath,
		p.Result.KeysPath,
	)
	return err
}
