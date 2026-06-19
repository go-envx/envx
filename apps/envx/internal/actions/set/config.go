package set

import "github.com/go-envx/envx/apps/envx/internal/config"

// -------------------------------------------------------------------------------------
// actionConfig holds the set action's config. There is no engine.Flags here: set
// mutates a single overlay file rather than resolving or merging an environment,
// so it only needs the shared root context.
type actionConfig struct {
	Global *config.Global
}
