package explain

import (
	"github.com/go-envx/envx/apps/envx/internal/actions"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: resolve settings, one engine.ResolveEnv, then
// the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	settings := actions.ResolveSettings(c.Global, p.Project, c.Settings, c.Changed)
	env, err := engine.ResolveEnv(&engine.Request{
		Config:   c.Global.Config,
		Project:  p.Project,
		Settings: settings,
	})
	if err != nil {
		return actionResult{}, err
	}
	return runAction(env, p, c.Reveal)
}
