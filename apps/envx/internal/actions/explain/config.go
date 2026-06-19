package explain

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the explain action's composed config: ConfigPath (the
// persistent --config flag), the raw flag values cobra binds (resolved via
// config.Resolve), the changed-flag handle, the --reveal toggle, and the
// --output format.
type actionConfig struct {
	ConfigPath *string
	Settings   engine.Settings
	Changed    config.FlagSet
	Reveal     bool
	Output     string
}
