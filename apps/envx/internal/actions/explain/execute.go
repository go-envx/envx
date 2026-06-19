package explain

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: load and resolve the manifest into an
// engine.Config, run the engine, then hand the result to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	m, err := manifest.New(*c.ConfigPath)
	if err != nil {
		return actionResult{}, err
	}
	ec, err := config.Resolve(m, p.Project, c.Settings, c.Changed)
	if err != nil {
		return actionResult{}, err
	}
	env, err := engine.Resolve(ec)
	if err != nil {
		return actionResult{}, err
	}
	return runAction(env, p, c.Reveal)
}
