package explain

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the explain action's composed config: the shared root context,
// the raw flag values cobra binds (resolved via actions.ResolveSettings), the
// changed-flag handle, the --reveal toggle, and the --output format.
type actionConfig struct {
	Global   *config.Global
	Settings engine.Settings
	Changed  config.FlagSet
	Reveal   bool
	Output   string
}
