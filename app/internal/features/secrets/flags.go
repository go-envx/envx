package secrets

import "github.com/go-envx/envx/app/internal/shared/flags"

// Flags local to secrets/keypair commands.
var (
	// Group narrows a bulk secret operation to one key group.
	Group = flags.Spec{
		Name:  "group",
		Short: "g",
		Usage: "limit to one key group (default: all groups)",
	}

	// Key narrows a bulk secret operation to one secret key.
	Key = flags.Spec{
		Name:  "key",
		Short: "k",
		Usage: "limit to one secret key (default: all keys)",
	}

	// NoConfirm skips the interactive confirmation after hidden input.
	NoConfirm = flags.Spec{
		Name:  "no-confirm",
		Usage: "skip the interactive confirmation prompt",
	}

	// Cipher selects the algorithm for ephemeral keypair generation.
	Cipher = flags.Spec{
		Name:  "cipher",
		Usage: "cipher algorithm (age|nacl-box)",
	}
)
