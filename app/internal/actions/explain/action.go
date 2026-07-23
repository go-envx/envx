package explain

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
)

// -------------------------------------------------------------------------------------

// actionParams are the positional inputs to the explain action.
type actionParams struct {
	// Project is the project name to resolve.
	Project string
	// Key is the env-var key to look up (case-insensitive).
	// An empty string means "explain all keys".
	Key string
}

// -------------------------------------------------------------------------------------

// actionResult is the data the explain action returns.
type actionResult struct {
	// Entries is the per-key explanation rows.
	Entries []actionResultEntry
}

// -------------------------------------------------------------------------------------

// actionResultEntry is one row of explain output.
type actionResultEntry struct {
	// Key is the resolved env-var key (uppercased).
	Key string
	// Value is the resolved value of the key.
	Value string
	// Source is the file that provided the resolved value.
	Source string
	// SourceKey is the original key in the source file that provided the resolved value.
	SourceKey string
	// Shadowed is the list of files that were overridden by the resolved value.
	Shadowed []string
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

	// explain the resolved keys
	return runAction(env, p)
}

// -------------------------------------------------------------------------------------

// runAction reads the resolved environment and origins to output sorted rows.
// A specific key that does not exist is an error.
func runAction(env *envmerge.Result, p actionParams) (actionResult, error) {
	// select the keys to explain
	var keys []string
	if p.Key != "" {
		key := strings.ToUpper(p.Key)
		if _, ok := env.Get(key); !ok {
			return actionResult{}, fmt.Errorf("key %q not found", key)
		}
		keys = []string{key}
	} else {
		keys = env.Keys()
	}

	// build a row per key with its value and origin
	entries := make([]actionResultEntry, 0, len(keys))
	for _, key := range keys {
		value, _ := env.Get(key)
		origin, _ := env.Origin(key)

		shadowed := make([]string, 0, len(origin.Shadowed))
		for _, s := range origin.Shadowed {
			shadowed = append(shadowed, s.File)
		}

		entries = append(entries, actionResultEntry{
			Key:       key,
			Value:     value,
			Source:    origin.Winner.File,
			SourceKey: origin.Winner.Key,
			Shadowed:  shadowed,
		})
	}
	return actionResult{Entries: entries}, nil
}
