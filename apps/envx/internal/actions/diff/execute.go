package diff

import "github.com/go-envx/envx/apps/envx/internal/engine"

// -------------------------------------------------------------------------------------
// execute is the imperative shell: it resolves the project once per environment
// (passing each side via Request.Environment) and feeds both results to the pure
// core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	left, err := engine.ResolveEnv(&engine.Request{
		Global:      *c.Global,
		Project:     p.Project,
		Environment: p.LeftEnv,
		Flags:       c.Flags,
		Changed:     c.Changed,
	})
	if err != nil {
		return actionResult{}, err
	}
	right, err := engine.ResolveEnv(&engine.Request{
		Global:      *c.Global,
		Project:     p.Project,
		Environment: p.RightEnv,
		Flags:       c.Flags,
		Changed:     c.Changed,
	})
	if err != nil {
		return actionResult{}, err
	}
	return runAction(left, right), nil
}
