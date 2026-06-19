package run

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the run action's composed config: the shared root context, the
// raw flag values cobra binds (resolved via actions.ResolveSettings), the
// changed-flag handle that drives precedence, and the run-local --overload
// toggle.
type actionConfig struct {
	Global   *config.Global
	Settings engine.Settings
	Changed  config.FlagSet
	Overload bool
}
