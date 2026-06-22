package engine

import "github.com/go-envx/envx/apps/envx/internal/settings"

// -------------------------------------------------------------------------------------
// Config is the engine's input contract: a plain data bag the caller fully
// populates (the config package builds it from a manifest plus CLI overrides).
// The engine knows nothing about envx.yaml, cobra, or precedence. Every field
// is optional and the engine fills in terminal defaults itself. The resolved
// knobs live in settings.Resolved, the shared value form every layer reads.
type Config struct {
	Dir          string   // workspace root; include paths resolve against it
	Includes     []string // one project's ordered namespace chain
	Environments []string // declared environments, for validating the target
	Settings     settings.Resolved
}
