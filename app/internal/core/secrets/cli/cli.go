// Package cli provides CLI commands for secrets and keypair management.
package cli

import (
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/lib/cliflags"
)

// Flags local to secrets/keypair commands.
var (
	// groupFlag narrows a bulk secret operation to one key group.
	groupFlag = cliflags.Spec[string]{
		Name:  "group",
		Short: "g",
		Usage: "limit to one key group (default: all groups)",
	}

	// keyFlag narrows a bulk secret operation to one secret key.
	keyFlag = cliflags.Spec[string]{
		Name:  "key",
		Short: "k",
		Usage: "limit to one secret key (default: all keys)",
	}

	// noConfirmFlag skips the interactive confirmation after hidden input.
	noConfirmFlag = cliflags.Spec[bool]{
		Name:  "no-confirm",
		Usage: "skip the interactive confirmation prompt",
	}

	// verboseFlag lists each changed secret in bulk operation output.
	verboseFlag = cliflags.Spec[bool]{
		Name:  "verbose",
		Short: "v",
		Usage: "print detailed output",
	}
)

// Factory defines the capabilities required for the secrets/keypair commands.
type Factory interface {
	NewSecretsService(configPath string) (*secrets.Service, error)
}
