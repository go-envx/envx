package diff

import (
	"github.com/go-envx/envx/apps/envx/internal/actions"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: it resolves the merge settings once, then
// resolves the project under each environment (overriding Settings.Env with each
// positional side) and feeds both results to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	settings := actions.ResolveSettings(c.Global, p.Project, c.Settings, c.Changed)

	leftSettings := settings
	leftSettings.Env = p.LeftEnv
	left, err := engine.ResolveEnv(&engine.Request{
		Config:   c.Global.Config,
		Project:  p.Project,
		Settings: leftSettings,
	})
	if err != nil {
		return actionResult{}, err
	}

	rightSettings := settings
	rightSettings.Env = p.RightEnv
	right, err := engine.ResolveEnv(&engine.Request{
		Config:   c.Global.Config,
		Project:  p.Project,
		Settings: rightSettings,
	})
	if err != nil {
		return actionResult{}, err
	}
	return runAction(left, right), nil
}
