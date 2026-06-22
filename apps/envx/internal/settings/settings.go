package settings

// -------------------------------------------------------------------------------------
// Resolved holds the fully-resolved env-resolution knobs the engine consumes — a
// plain value struct with no knowledge of CLI precedence or YAML. Zero values are
// valid: an empty Env falls back to DefaultEnv and the bool/string knobs default
// to off. The config package produces it by layering File values and CLI input.
type Resolved struct {
	Env             string
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}

// -------------------------------------------------------------------------------------
// File is the settings block as written in envx.yaml — both the global block and
// each project's block. Booleans are pointers so an explicitly-set false is
// distinguishable from "unset", which the config precedence chain relies on when
// layering project over global over CLI input.
type File struct {
	Overload        *bool  `yaml:"overload"`
	Strict          *bool  `yaml:"strict"`
	Prefix          string `yaml:"prefix"`
	Suffix          string `yaml:"suffix"`
	NamespacePrefix *bool  `yaml:"namespace_prefix"`
	Env             string `yaml:"env"`
}

// -------------------------------------------------------------------------------------
// DefaultEnv is the environment used when none is selected via flag, ENVX_ENV, or
// a manifest env setting. It is the single non-zero terminal default; every other
// setting defaults to its Go zero value (Strict/NamespacePrefix=false, Prefix/
// Suffix=""). The engine applies it so a bare engine.Config still resolves.
const DefaultEnv = "development"
