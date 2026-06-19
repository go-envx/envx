package run

import (
	"context"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: load and resolve the manifest into an
// engine.Config, run the engine, resolve the overload setting (flag >
// ENVX_OVERLOAD), then exec the child process with the merged environment.
func execute(ctx context.Context, p actionParams, c *actionConfig) error {
	m, err := manifest.New(*c.ConfigPath)
	if err != nil {
		return err
	}
	ec, err := config.Resolve(m, p.Project, c.Settings, c.Changed)
	if err != nil {
		return err
	}
	env, err := engine.Resolve(ec)
	if err != nil {
		return err
	}
	overload := config.NewResolver().Bool(flags.Overload, c.Changed, c.Overload)
	return runAction(ctx, env, p, overload)
}
