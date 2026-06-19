package set

import "github.com/go-envx/envx/apps/envx/internal/config"

// -------------------------------------------------------------------------------------
// actionConfig holds the set action's config. There is no engine.Settings here:
// set mutates a single overlay file rather than resolving or merging an
// environment, so it only needs the shared root context plus the raw --env value
// and the changed-flag handle used to resolve the target environment.
type actionConfig struct {
	Global  *config.Global
	Env     string
	Changed config.FlagSet
}
