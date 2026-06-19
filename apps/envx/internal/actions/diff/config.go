package diff

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the diff action's composed config: the shared root context,
// the engine settings, the changed-flag handle, the --reveal toggle, and the
// --output format.
type actionConfig struct {
	Global  *config.Global
	Flags   engine.Flags
	Changed config.FlagSet
	Reveal  bool
	Output  string
}
