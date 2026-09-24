package cli

import "github.com/go-envx/envx/app/internal/utils/cliflags"

// Flags local to emit command.
var (
	// Target selects the output shape the emit command renders to.
	Target = cliflags.FlagSpec{
		Name:  "target",
		Short: "t",
		Usage: "output shape: k8s|k8s-bundle|json|dotenv",
	}

	// Only restricts the emitted output to one slice of the environment:
	// "secrets" for the secret-derived values or "config" for the plain ones.
	Only = cliflags.FlagSpec{
		Name:  "only",
		Usage: "restrict output to one slice: secrets|config (default: all values)",
	}

	// Name overrides the base for k8s resource names (default: the project name).
	Name = cliflags.FlagSpec{
		Name:  "name",
		Short: "n",
		Usage: "base for k8s resource names (default: project name; k8s targets only)",
	}

	// Key overrides the k8s-bundle data key / mounted filename.
	Key = cliflags.FlagSpec{
		Name:  "key",
		Usage: "k8s-bundle data key / filename (default: config.json or secrets.json)",
	}

	// Output writes the rendered output to a file instead of stdout.
	Output = cliflags.FlagSpec{
		Name:  "output",
		Short: "o",
		Usage: "write output to this file instead of stdout",
	}
)
