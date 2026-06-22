// Package settings is the single source of truth for every envx setting. It owns
// the Spec catalog (flag name, shorthand, ENVX_* fallback, usage, and default)
// that the config binders and resolver read, plus the typed structs each layer
// consumes: Resolved (the value form the engine merges), File (the pointer/yaml
// form the manifest parses), and the one non-zero terminal default, DefaultEnv.
//
// It is a pure leaf importing only fmt — no cobra, no yaml, no engine. Binding
// specs onto cobra commands lives in the config package; the engine and manifest
// packages read the structs. Because settings has no behavior and no internal
// dependencies, every layer may import it without coupling.
package settings
