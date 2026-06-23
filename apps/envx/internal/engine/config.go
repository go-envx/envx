package engine

// -------------------------------------------------------------------------------------
// Config is the engine's input contract: a plain data bag the caller fully
// populates (the config package builds it from a manifest plus CLI overrides).
// The engine knows nothing about envx.yaml, cobra, or precedence. Every field
// is optional and the engine fills in terminal defaults itself. The resolved
// knobs live in Settings, the value form the engine merges.
type Config struct {
	Dir          string   // workspace root; include paths resolve against it
	Includes     []string // one project's ordered namespace chain
	Environments []string // declared environments, for validating the target
	Settings     Settings
}

// -------------------------------------------------------------------------------------
// Settings holds the fully-resolved env-resolution knobs the engine consumes — a
// plain value struct with no knowledge of CLI precedence or YAML. Zero values are
// valid: an empty Env falls back to the first declared environment and the
// bool/string knobs default to off. The config package produces it by layering the
// declared schema.Settings over CLI input. It is the effective counterpart to the
// declared schema.Settings.
type Settings struct {
	Env             string
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}
