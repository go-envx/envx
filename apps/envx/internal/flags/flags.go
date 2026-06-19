// Package flags is the single source of truth for every envx CLI flag. It is a
// pure leaf package that imports nothing from the rest of the codebase (and no
// CLI framework), so config, engine, cli, and the actions can all read the same
// catalog without pulling in cobra or pflag. Registration happens at the cobra
// edge; this package only *describes* flags.
package flags

import "fmt"

// -------------------------------------------------------------------------------------
// Spec is one flag's identity. Both registration (cmd.Flags().XVarP) and the
// config resolver read the SAME Spec, so a flag's name and its ENVX_* env-var
// fallback are defined exactly once and can never drift apart.
type Spec struct {
	Name  string // --<Name>
	Short string // -<Short> ("" = none)
	Env   string // ENVX_* fallback ("" = none)
	Usage string // human-readable description
}

// -------------------------------------------------------------------------------------
// Catalog every envx flag in one block.
var (
	// Config selects the manifest path (auto-discovered when unset).
	Config = Spec{
		Name:  "config",
		Env:   "ENVX_CONFIG",
		Usage: "path to envx.yaml",
	}

	// Env selects the target environment.
	Env = Spec{
		Name:  "env",
		Short: "E",
		Env:   "ENVX_ENV",
		Usage: "target environment",
	}

	// Strict requires every overlay file in the namespace chain to exist.
	Strict = Spec{
		Name:  "strict",
		Env:   "ENVX_STRICT",
		Usage: "require all overlay files to exist",
	}

	// Prefix is prepended to every resolved env-var key.
	Prefix = Spec{
		Name:  "prefix",
		Env:   "ENVX_PREFIX",
		Usage: "prefix prepended to every key",
	}

	// Suffix is appended to every resolved env-var key.
	Suffix = Spec{
		Name:  "suffix",
		Env:   "ENVX_SUFFIX",
		Usage: "suffix appended to every key",
	}

	// NamespacePrefix prefixes each key with its namespace name.
	NamespacePrefix = Spec{
		Name:  "namespace-prefix",
		Env:   "ENVX_NAMESPACE_PREFIX",
		Usage: "prefix each key with its namespace",
	}

	// Overload lets file values override existing OS env vars.
	Overload = Spec{
		Name:  "overload",
		Env:   "ENVX_OVERLOAD",
		Usage: "file values override OS env vars",
	}

	// Reveal prints values in plaintext instead of masking them.
	Reveal = Spec{
		Name:  "reveal",
		Usage: "print values instead of masking",
	}

	// Output selects the rendering format for tabular commands.
	Output = Spec{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}
)

// -------------------------------------------------------------------------------------
// HelpText renders the usage string with the env-var hint appended when the
// flag has an ENVX_* fallback, e.g. "target environment (env: ENVX_ENV)". The
// result is used directly as the cobra usage string.
func (s Spec) HelpText() string {
	if s.Env == "" {
		return s.Usage
	}
	return fmt.Sprintf("%s (env: %s)", s.Usage, s.Env)
}
