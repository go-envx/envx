package engine

// -------------------------------------------------------------------------------------
// Config is the engine's input contract: a plain data bag the caller fully
// populates (the config package builds it from a manifest plus CLI overrides).
// The engine knows nothing about envx.yaml, cobra, or precedence. Every field
// is optional and the engine fills in terminal defaults itself.
type Config struct {
	Dir          string   // workspace root; include paths resolve against it
	Includes     []string // one project's ordered namespace chain
	Environments []string // declared environments, for validating the target
	Settings     Settings
}

// -------------------------------------------------------------------------------------
// Settings holds the resolved env-resolution knobs the engine consumes — a plain
// data struct with no knowledge of external callers or config precedence. Zero
// values are valid: an empty Env falls back to DefaultEnv and the bool/string
// knobs default to off.
type Settings struct {
	Env             string
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}

// -------------------------------------------------------------------------------------
// DefaultEnv is the environment used when none is selected via flag, ENVX_ENV,
// or a manifest env setting. The engine owns this terminal fallback so a bare
// engine.Config still resolves.
const DefaultEnv = "development"
