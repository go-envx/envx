package run

import (
	"context"

	"github.com/go-envx/envx/apps/envx/internal/actions"
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: resolve the settings precedence and the
// environment, then the overload setting (flag > ENVX_OVERLOAD), then exec the
// child process with it.
func execute(ctx context.Context, p actionParams, c *actionConfig) error {
	settings := actions.ResolveSettings(c.Global, p.Project, c.Settings, c.Changed)
	env, err := engine.ResolveEnv(&engine.Request{
		Config:   c.Global.Config,
		Project:  p.Project,
		Settings: settings,
	})
	if err != nil {
		return err
	}
	overload := config.NewResolver().Bool(flags.Overload, c.Changed, c.Overload)
	return runAction(ctx, env, p, overload)
}
