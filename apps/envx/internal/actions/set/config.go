package set

import "github.com/go-envx/envx/apps/envx/internal/config"

// -------------------------------------------------------------------------------------
// actionConfig holds the set action's config. There is no engine.Settings here:
// set mutates a single overlay file rather than resolving or merging an
// environment, so it only needs the persistent --config path plus the raw --env
// value and the changed-flag handle used to resolve the target environment.
type actionConfig struct {
	ConfigPath *string
	Env        string
	Changed    config.FlagSet
}
