package get

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the get action's composed, typed config. ConfigPath points at
// the persistent --config flag (loaded into a manifest in execute), Settings
// holds the raw flag values cobra binds (resolved via config.Resolve), and
// Changed is the behavioral handle that drives settings precedence.
type actionConfig struct {
	ConfigPath *string
	Settings   engine.Settings
	Changed    config.FlagSet
}
