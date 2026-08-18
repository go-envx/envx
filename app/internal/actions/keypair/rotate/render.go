package rotate

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
)

// renderParams bundles the output sink and safe rotation result.
type renderParams struct {
	// Printer is the styled output layer for the safe rotation summary.
	Printer *printer.Printer
	// Result is the safe rotation metadata to render.
	Result actionResult
}

// render prints the new public identity, store paths, and the re-encrypted
// secret identities without any private-key material or secret values.
func render(p *renderParams) error {
	keypairs := p.Result.Result.Keypairs
	if len(keypairs) == 0 {
		return fmt.Errorf("rotation reported no keypair change")
	}
	rotated := keypairs[0]

	var b strings.Builder
	fmt.Fprintf(
		&b,
		"Rotated keypair for group %q:\n"+
			"  public key: %s\n"+
			"  secrets store: %s\n"+
			"  private key file: %s\n"+
			"  re-encrypted %s",
		rotated.Group,
		rotated.PublicKey,
		p.Result.SecretsPath,
		p.Result.KeysPath,
		str.Pluralize(len(p.Result.Result.Secrets), "secret", "secrets"),
	)

	for _, secret := range p.Result.Result.Secrets {
		fmt.Fprintf(&b, "\n    %s/%s", secret.Group, secret.Key)
	}
	return p.Printer.LogMessage(b.String())
}
