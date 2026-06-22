package get

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionParams are the positional inputs to the get action.
type actionParams struct {
	Project string
	Key     string
}

// -------------------------------------------------------------------------------------
// actionResult is the data the get action renders: the resolved value and the
// file it came from.
type actionResult struct {
	Value  string
	Source string
}

// -------------------------------------------------------------------------------------
// actionConfig is the get action's composed config: the shared resolution input
// (config path, raw flag values, changed-flag handle) gathered at the cobra edge
// and turned into an engine.Config by config.Resolve.
type actionConfig struct {
	config.Input
}

// -------------------------------------------------------------------------------------
// execute is the imperative shell: resolve the input into an engine.Config, build
// the merged environment, and hand the immutable result to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	ec, err := config.Resolve(&c.Input, p.Project)
	if err != nil {
		return actionResult{}, err
	}
	env, err := engine.Build(ec)
	if err != nil {
		return actionResult{}, err
	}
	return runAction(env, p)
}

// -------------------------------------------------------------------------------------
// runAction is the pure core: a case-insensitive lookup against an already
// resolved environment. Plain data in, plain data out; no engine call, no I/O,
// no cobra.
func runAction(env *engine.Result, p actionParams) (actionResult, error) {
	key := strings.ToUpper(p.Key)
	val, ok := env.Get(key)
	if !ok {
		return actionResult{}, fmt.Errorf("key %q not found", key)
	}
	origin, _ := env.Origin(key)
	return actionResult{Value: val, Source: origin.Winner.File}, nil
}
