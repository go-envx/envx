package schema

import "fmt"

// FlagSpec is one setting's CLI identity. Flag registration and config resolution
// read the SAME FlagSpec, so a setting's flag name and its ENVX_* env-var fallback
// are defined exactly once and can never drift apart.
type FlagSpec struct {
	// Name is the long-form flag name, e.g. "env" for "--env".
	Name string
	// Short is the short-form flag name, e.g. "E" for "-E".
	Short string
	// Env is the ENVX_* environment variable fallback ("" = none).
	Env string
	// Usage is the human-readable description for the flag usage string.
	Usage string
	// DefaultString is a string flag's fallback value, used when the flag is unset
	// (the zero value "" when not specified).
	DefaultString string
	// DefaultBool is a bool flag's fallback value, used when the flag is unset (the
	// zero value false when not specified).
	DefaultBool bool
}

// Catalog every envx setting and shared CLI flag identity in one block.
var (
	// Cipher selects the algorithm for ephemeral keypair generation.
	Cipher = FlagSpec{
		Name:  "cipher",
		Usage: "cipher algorithm (age|nacl-box)",
	}

	// Config selects the manifest path (auto-discovered when unset).
	Config = FlagSpec{
		Name:  "config",
		Env:   "ENVX_CONFIG",
		Usage: "path to envx.yaml",
	}

	// Delimiter joins a list-valued leaf into a single env var.
	Delimiter = FlagSpec{
		Name:  "delimiter",
		Env:   "ENVX_DELIMITER",
		Usage: `string used to join list values (default ",")`,
	}

	// Env selects the target environment. It advertises no static default because
	// the terminal fallback is the first declared environment, known only once the
	// manifest is loaded.
	Env = FlagSpec{
		Name:  "env",
		Short: "E",
		Env:   "ENVX_ENV",
		Usage: "target environment (defaults to first declared environment in envx.yaml)",
	}

	// Group narrows a bulk secret operation to one key group (default: all groups).
	Group = FlagSpec{
		Name:  "group",
		Short: "g",
		Usage: "limit to one key group (default: all groups)",
	}

	// Key narrows a bulk secret operation to one secret key (default: all keys).
	Key = FlagSpec{
		Name:  "key",
		Short: "k",
		Usage: "limit to one secret key (default: all keys)",
	}

	// NamespacePrefix prefixes each key with its namespace name.
	NamespacePrefix = FlagSpec{
		Name:  "namespace-prefix",
		Env:   "ENVX_NAMESPACE_PREFIX",
		Usage: "prefix each key with its namespace",
	}

	// NoConfirm skips the interactive confirmation after hidden input.
	NoConfirm = FlagSpec{
		Name:  "no-confirm",
		Usage: "skip the interactive confirmation prompt",
	}

	// Output selects the rendering format for tabular commands.
	Output = FlagSpec{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}

	// Overload lets file values override existing OS env vars.
	Overload = FlagSpec{
		Name:  "overload",
		Env:   "ENVX_OVERLOAD",
		Usage: "file values override OS env vars",
	}

	// Prefix is prepended to every resolved env-var key.
	Prefix = FlagSpec{
		Name:  "prefix",
		Env:   "ENVX_PREFIX",
		Usage: "prefix prepended to every key",
	}

	// RequireOverlays requires every environment overlay file in the namespace to exist.
	RequireOverlays = FlagSpec{
		Name:  "require-overlays",
		Env:   "ENVX_REQUIRE_OVERLAYS",
		Usage: "require all environment overlay files to exist",
	}

	// Reveal decrypts referenced secret values instead of masking them.
	Reveal = FlagSpec{
		Name:  "reveal",
		Usage: "decrypt secret references instead of masking them",
	}

	// Suffix is appended to every resolved env-var key.
	Suffix = FlagSpec{
		Name:  "suffix",
		Env:   "ENVX_SUFFIX",
		Usage: "suffix appended to every key",
	}

	// Verbose prints additional detail in command output.
	Verbose = FlagSpec{
		Name:  "verbose",
		Short: "v",
		Usage: "print detailed output",
	}
)

// HelpText renders the usage string with the env-var hint appended when the
// setting has an ENVX_* fallback, e.g. "target environment (env: ENVX_ENV)". The
// result is used directly as the cobra usage string.
func (s *FlagSpec) HelpText() string {
	if s.Env == "" {
		return s.Usage
	}
	return fmt.Sprintf("%s (env: %s)", s.Usage, s.Env)
}
