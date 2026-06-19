package get

import "github.com/go-envx/envx/apps/envx/internal/engine"

// -------------------------------------------------------------------------------------
// execute is the imperative shell: it resolves the environment (the only
// effect) and hands the immutable result to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	env, err := engine.ResolveEnv(&engine.Request{
		Global:  *c.Global,
		Project: p.Project,
		Flags:   c.Flags,
		Changed: c.Changed,
	})
	if err != nil {
		return actionResult{}, err
	}
	return runAction(env, p)
}
