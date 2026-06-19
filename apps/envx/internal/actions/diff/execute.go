package diff

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: it loads the manifest and resolves the merge
// settings once, then runs the engine under each environment (overriding
// Settings.Env with each positional side) and feeds both results to the pure
// core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	m, err := manifest.New(*c.ConfigPath)
	if err != nil {
		return actionResult{}, err
	}
	ec, err := config.Resolve(m, p.Project, c.Settings, c.Changed)
	if err != nil {
		return actionResult{}, err
	}

	ec.Settings.Env = p.LeftEnv
	left, err := engine.Resolve(ec)
	if err != nil {
		return actionResult{}, err
	}

	ec.Settings.Env = p.RightEnv
	right, err := engine.Resolve(ec)
	if err != nil {
		return actionResult{}, err
	}
	return runAction(left, right), nil
}
