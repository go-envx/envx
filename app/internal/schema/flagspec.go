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
	// Absolute renders source paths absolutely instead of relative to envx.yaml.
	Absolute = FlagSpec{
		Name:  "absolute",
		Usage: "show absolute source paths instead of paths relative to envx.yaml",
	}

	// Cipher selects the algorithm for ephemeral keypair generation.
	Cipher = FlagSpec{
		Name:  "cipher",
		Usage: "cipher algorithm (age|nacl-box)",
	}

	// Config selects the manifest path (auto-discovered when unset).
	Config = FlagSpec{
		Name:  "config",
		Env:   "ENVX_CONFIG",
		Usage: "path to envx.yaml, or a directory containing it",
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

	// EmitTarget selects the output shape the emit command renders to.
	EmitTarget = FlagSpec{
		Name:  "target",
		Short: "t",
		Usage: "output shape: k8s|k8s-bundle|json|dotenv",
	}

	// EmitOnly restricts the emitted output to one slice of the environment:
	// "secrets" for the secret-derived values or "config" for the plain ones.
	// Unset emits everything.
	EmitOnly = FlagSpec{
		Name:  "only",
		Usage: "restrict output to one slice: secrets|config (default: all values)",
	}

	// EmitName overrides the base for k8s resource names (default: the project
	// name); the render appends the slice suffix. k8s targets only.
	EmitName = FlagSpec{
		Name:  "name",
		Short: "n",
		Usage: "base for k8s resource names (default: project name; k8s targets only)",
	}

	// EmitKey overrides the k8s-bundle data key, which is also the mounted
	// filename; its .json/.env extension picks the body format.
	EmitKey = FlagSpec{
		Name:  "key",
		Usage: "k8s-bundle data key / filename (default: config.json or secrets.json)",
	}

	// EmitOutput writes the rendered output to a file instead of stdout.
	EmitOutput = FlagSpec{
		Name:  "output",
		Short: "o",
		Usage: "write output to this file instead of stdout",
	}

	// Group narrows a bulk secret operation to one key group (default: all groups).
	Group = FlagSpec{
		Name:  "group",
		Short: "g",
		Usage: "limit to one key group (default: all groups)",
	}

	// IgnoreErrors downgrades resolution failures to warnings and omits the failing
	// keys so the child process still starts.
	IgnoreErrors = FlagSpec{
		Name:  "ignore-errors",
		Usage: "warn on unresolved values and omit them instead of aborting",
	}

	// Key narrows a bulk secret operation to one secret key (default: all keys).
	Key = FlagSpec{
		Name:  "key",
		Short: "k",
		Usage: "limit to one secret key (default: all keys)",
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

	// Out selects the destination directory a bundle is written to.
	Out = FlagSpec{
		Name:  "out",
		Usage: "destination directory for the packed workspace",
	}

	// PackEnv selects one or more environments to include in a pack bundle. It is
	// repeatable and distinct from --env: pack copies several environments'
	// overlays into one bundle rather than resolving a single environment.
	PackEnv = FlagSpec{
		Name:  "env",
		Short: "e",
		Usage: "environment to include (repeatable; default: all declared)",
	}

	// PackProject narrows a pack bundle to one or more projects' includes.
	PackProject = FlagSpec{
		Name:  "project",
		Short: "p",
		Usage: "limit the bundle to this project's includes (repeatable; default: all)",
	}

	// PackForce replaces an existing non-empty output directory instead of
	// refusing it, clearing the directory before the bundle is written.
	PackForce = FlagSpec{
		Name:  "force",
		Usage: "replace the output directory if it already exists and is not empty",
	}

	// Overload lets file values override existing OS env vars.
	Overload = FlagSpec{
		Name:  "overload",
		Env:   "ENVX_OVERLOAD",
		Usage: "file values override OS env vars",
	}

	// ReferencePattern overrides the {{VAR}} reference syntax with a regex.
	ReferencePattern = FlagSpec{
		Name:  "reference-pattern",
		Env:   "ENVX_REFERENCE_PATTERN",
		Usage: "regex overriding the {{VAR}} reference syntax (group 1 is the name)",
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

	// Strict fails validation on warnings as well as errors.
	Strict = FlagSpec{
		Name:  "strict",
		Usage: "fail on warnings as well as errors",
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
