package rotate

import (
	"fmt"
	"io"
)

// renderParams bundles the output sink and safe rotation result.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
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

	if _, err := fmt.Fprintf(
		p.Writer,
		"Rotated keypair for group %q:\n"+
			"  public key: %s\n"+
			"  secrets store: %s\n"+
			"  private key file: %s\n"+
			"  re-encrypted %d secret(s)\n",
		rotated.Group,
		rotated.PublicKey,
		p.Result.SecretsPath,
		p.Result.KeysPath,
		len(p.Result.Result.Secrets),
	); err != nil {
		return err
	}

	for _, secret := range p.Result.Result.Secrets {
		if _, err := fmt.Fprintf(
			p.Writer, "    %s/%s\n", secret.Group, secret.Key,
		); err != nil {
			return err
		}
	}
	return nil
}
