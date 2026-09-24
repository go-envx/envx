package print

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/resources/cipher"
)

// actionResult contains the explicitly requested ephemeral keypair.
type actionResult struct {
	// Keypair contains the public and private values for manual handoff.
	Keypair cipher.Keypair
}

// execute generates a keypair through the selected cipher without opening or
// mutating the workspace store or private-key file.
func execute(in *core.Input, cipherName string) (actionResult, error) {
	selectedCipher, err := resolveCipher(in, cipherName)
	if err != nil {
		return actionResult{}, err
	}
	pair, err := selectedCipher.Keypair()
	if err != nil {
		return actionResult{}, fmt.Errorf("generating keypair: %w", err)
	}
	return actionResult{Keypair: pair}, nil
}

// resolveCipher prefers the explicit command flag, then uses workspace
// configuration and finally the application's default through config.
func resolveCipher(in *core.Input, cipherName string) (cipher.Cipher, error) {
	if cipherName == "" {
		return core.NewConfiguredCipher(in)
	}
	selectedCipher, err := cipher.New(cipher.Params{
		Algorithm: cipher.Algorithm(cipherName),
	})
	if err != nil {
		return nil, fmt.Errorf("creating cipher %q: %w", cipherName, err)
	}
	return selectedCipher, nil
}
