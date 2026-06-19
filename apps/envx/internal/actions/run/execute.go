package run

import (
	"context"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: resolve the environment and the overload
// setting (flag > ENVX_OVERLOAD), then exec the child process with it.
func execute(ctx context.Context, p actionParams, c *actionConfig) error {
	env, err := engine.ResolveEnv(&engine.Request{
		Global:  *c.Global,
		Project: p.Project,
		Flags:   c.Flags,
		Changed: c.Changed,
	})
	if err != nil {
		return err
	}
	overload := config.NewResolver().Bool(flags.Overload, c.Changed, c.Overload)
	return runAction(ctx, env, p, overload)
}
