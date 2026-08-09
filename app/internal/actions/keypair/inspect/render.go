package inspect

import (
	"fmt"
	"io"

	"github.com/go-envx/envx/app/internal/secrets"
)

// -------------------------------------------------------------------------------------

// renderParams bundles the output sink and keypair inspection result.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the public key metadata and private-key status to render.
	Result secrets.KeypairMetadata
}

// -------------------------------------------------------------------------------------

// render prints public metadata and private-key status only.
func render(p *renderParams) error {
	_, err := fmt.Fprintf(
		p.Writer,
		"Keypair for group %q:\n"+
			"  public key: %s\n"+
			"  private key status: %s\n",
		p.Result.Group,
		p.Result.PublicKey,
		p.Result.PrivateKeyStatus,
	)
	return err
}
