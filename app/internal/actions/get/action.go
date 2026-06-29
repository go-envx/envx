package get

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
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

// actionConfig is the get action's configurable surface: the envmerge settings it
// resolves before merging the environment.
type actionConfig struct {
	// Env is the target environment to resolve.
	Env string
	// Strict requires every overlay file in the namespace chain to exist.
	Strict bool
	// Prefix is prepended to every resolved key.
	Prefix string
	// Suffix is appended to every resolved key.
	Suffix string
	// NamespacePrefix prefixes each key with its namespace name.
	NamespacePrefix bool
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

// execute is the imperative shell: resolve the input into an envmerge.Params, build
// the merged environment, and hand the result to the pure core.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// resolve the input config
	resolved, err := config.Resolve(in, p.Project)
	if err != nil {
		return actionResult{}, err
	}

	// build the merged environment
	env, err := envmerge.Build(resolved.Envmerge)
	if err != nil {
		return actionResult{}, err
	}

	// look up the requested key
	return runAction(env, p)
}

// -------------------------------------------------------------------------------------

// runAction performs a case-insensitive lookup against the specified environment,
// returning a single value for the given key.
func runAction(env *envmerge.Result, p actionParams) (actionResult, error) {
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
