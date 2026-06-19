package get

import (
	"github.com/go-envx/envx/apps/envx/internal/actions"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: it resolves the settings precedence, resolves
// the environment (the only effect), and hands the immutable result to the pure
// core.
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
	return runAction(env, p)
}
