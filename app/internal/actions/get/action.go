package get

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
)

// actionParams are the inputs to the get action.
type actionParams struct {
	// Project is the project name to resolve.
	Project string
	// Key is the env-var key to look up (case-insensitive).
	Key string
	// Reveal controls whether secret references are resolved to plaintext.
	Reveal bool
}

// actionResult is the data the get action returns.
type actionResult struct {
	// Value is the resolved value of the requested key.
	Value string
	// Source is the file that provided the resolved value.
	Source string
}

// execute is the imperative shell: resolve the input into an envmerge.Params, build
// the merged environment, and hand the result to the pure core. Secret references
// are masked unless p.Reveal is set.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// resolve the input config
	resolved, err := config.ResolveProject(in, p.Project, p.Reveal)
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

// runAction performs a case-insensitive lookup against the specified environment,
// returning a single value for the given key. A dangling reference behind a
// different key is ignored; only the requested key's own resolution failure is
// reported.
func runAction(env *envmerge.Result, p actionParams) (actionResult, error) {
	// look up the key case-insensitively
	key := strings.ToUpper(p.Key)
	val, ok := env.Get(key)
	if !ok {
		// surface the requested key's own resolution failure before reporting it
		// as absent, so a dangling secret reports why it could not resolve.
		if err := env.Err(key); err != nil {
			return actionResult{}, err
		}
		return actionResult{}, fmt.Errorf("key %q not found", key)
	}

	// report the value and its source file
	origin, _ := env.Origin(key)
	return actionResult{Value: val, Source: origin.Winner.File}, nil
}
