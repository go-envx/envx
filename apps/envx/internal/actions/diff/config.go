package diff

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionConfig is the diff action's composed config: ConfigPath (the persistent
// --config flag), the raw merge-setting flag values cobra binds (resolved via
// config.Resolve; the environments come from positional args, not --env), the
// changed-flag handle, the --reveal toggle, and the --output format.
type actionConfig struct {
	ConfigPath *string
	Settings   engine.Settings
	Changed    config.FlagSet
	Reveal     bool
	Output     string
}
