// Package cli provides CLI commands for secrets and keypair management.
package cli

import (
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/shared/flags"
)

// Flags local to secrets/keypair commands.
var (
	// groupFlag narrows a bulk secret operation to one key group.
	groupFlag = flags.Spec[string]{
		Name:  "group",
		Short: "g",
		Usage: "limit to one key group (default: all groups)",
	}

	// keyFlag narrows a bulk secret operation to one secret key.
	keyFlag = flags.Spec[string]{
		Name:  "key",
		Short: "k",
		Usage: "limit to one secret key (default: all keys)",
	}

	// noConfirmFlag skips the interactive confirmation after hidden input.
	noConfirmFlag = flags.Spec[bool]{
		Name:  "no-confirm",
		Usage: "skip the interactive confirmation prompt",
	}
)

// Factory defines the capabilities required for the secrets/keypair commands.
type Factory interface {
	SecretsService(configPath string) (*secrets.Service, error)
}
