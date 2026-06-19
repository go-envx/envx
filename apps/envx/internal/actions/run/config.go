package run

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the run action's composed config: ConfigPath (the persistent
// --config flag), the raw flag values cobra binds (resolved via config.Resolve),
// the changed-flag handle that drives precedence, and the run-local --overload
// toggle.
type actionConfig struct {
	ConfigPath *string
	Settings   engine.Settings
	Changed    config.FlagSet
	Overload   bool
}
