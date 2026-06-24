package get

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/engine"
)

// -------------------------------------------------------------------------------------

// actionParams are the positional inputs to the get action.
type actionParams struct {
	// Project is the project name to resolve.
	Project string
	// Key is the env-var key to look up (case-insensitive).
	Key string
}

// -------------------------------------------------------------------------------------

// actionConfig is the get action's composed config.
type actionConfig struct {
	config.Input
}

// -------------------------------------------------------------------------------------

// actionResult is the data the get action returns.
type actionResult struct {
	// Value is the resolved value of the requested key.
	Value string
	// Source is the file that provided the resolved value.
	Source string
}

// -------------------------------------------------------------------------------------

// execute is the imperative shell: resolve the input into an engine.Config, build
// the merged environment, and hand the result to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	// resolve the input config
	ec, err := config.Resolve(&c.Input, p.Project)
	if err != nil {
		return actionResult{}, err
	}

	// build the merged environment
	env, err := engine.Build(ec)
	if err != nil {
		return actionResult{}, err
	}

	// look up the requested key
	return runAction(env, p)
}

// -------------------------------------------------------------------------------------

// runAction performs a case-insensitive lookup against the specified environment,
// returning a single value for the given key.
func runAction(env *engine.Result, p actionParams) (actionResult, error) {
	// look up the key case-insensitively
	key := strings.ToUpper(p.Key)
	val, ok := env.Get(key)
	if !ok {
		return actionResult{}, fmt.Errorf("key %q not found", key)
	}

	// report the value and its source file
	origin, _ := env.Origin(key)
	return actionResult{Value: val, Source: origin.Winner.File}, nil
}
