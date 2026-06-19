package get

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the get action's composed, typed config. Field types reflect
// each value's role: Global is a shared pointer filled by cli.PreRunE, Settings
// holds the raw flag values cobra binds (resolved via actions.ResolveSettings),
// and Changed is the behavioral handle that drives settings precedence.
type actionConfig struct {
	Global   *config.Global
	Settings engine.Settings
	Changed  config.FlagSet
}
