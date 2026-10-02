// Package cli provides CLI commands for environment resolution,
// inspection, and modification.
package cli

import (
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
)

// Flags local to environment commands.
var (
	// envFlag selects the target environment.
	envFlag = flags.Spec[string]{
		Name:  "env",
		Short: "E",
		Env:   "ENVX_ENV",
		Usage: "target environment (defaults to first declared environment in envx.yaml)",
	}

	// requireOverlaysFlag requires every environment overlay file in the namespace to
	// exist.
	requireOverlaysFlag = flags.Spec[bool]{
		Name:  "require-overlays",
		Env:   "ENVX_REQUIRE_OVERLAYS",
		Usage: "require all environment overlay files to exist",
	}

	// prefixFlag is prepended to every resolved env-var key.
	prefixFlag = flags.Spec[string]{
		Name:  "prefix",
		Env:   "ENVX_PREFIX",
		Usage: "prefix prepended to every key",
	}

	// suffixFlag is appended to every resolved env-var key.
	suffixFlag = flags.Spec[string]{
		Name:  "suffix",
		Env:   "ENVX_SUFFIX",
		Usage: "suffix appended to every key",
	}

	// delimiterFlag joins a list-valued leaf into a single env var.
	delimiterFlag = flags.Spec[string]{
		Name:  "delimiter",
		Env:   "ENVX_DELIMITER",
		Usage: `string used to join list values (default ",")`,
	}

	// overloadFlag lets file values override existing OS env vars.
	overloadFlag = flags.Spec[bool]{
		Name:  "overload",
		Env:   "ENVX_OVERLOAD",
		Usage: "file values override OS env vars",
	}

	// referencePatternFlag overrides the {{VAR}} reference syntax with a regex.
	referencePatternFlag = flags.Spec[string]{
		Name:  "reference-pattern",
		Env:   "ENVX_REFERENCE_PATTERN",
		Usage: "regex overriding the {{VAR}} reference syntax (group 1 is the name)",
	}

	// revealFlag decrypts referenced secret values instead of masking them.
	revealFlag = flags.Spec[bool]{
		Name:  "reveal",
		Usage: "decrypt secret references instead of masking them",
	}

	// absoluteFlag renders source paths absolutely instead of relative to envx.yaml.
	absoluteFlag = flags.Spec[bool]{
		Name:  "absolute",
		Usage: "show absolute source paths instead of paths relative to envx.yaml",
	}
)

// Factory defines the capabilities required for the environment commands.
type Factory interface {
	EnvService(configPath string) (*env.Service, error)
}
