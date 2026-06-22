// Package flags is the single source of truth for every envx CLI flag. It owns
// both halves of a flag's lifecycle: the Spec catalog (name, shorthand, ENVX_*
// fallback, usage, and default) that the config and manifest packages read for
// precedence resolution, and the cobra registration constructors (New*Flag and
// NewEngineFlags) that bind those same specs onto commands. Because it registers
// flags it imports cobra and engine; the resolution layers still read the same
// catalog, so a flag's name and its env-var fallback can never drift apart.
package flags
