package explain

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionParams are the inputs to explain. An empty Key means "explain every
// key".
type actionParams struct {
	Project string
	Key     string
}

// -------------------------------------------------------------------------------------
// entry is one row of explain output: the resolved key/value, the winning
// source file and its original nested key, and the files it shadowed.
type entry struct {
	Key       string
	Value     string
	Source    string
	SourceKey string
	Shadowed  []string
}

// -------------------------------------------------------------------------------------
// actionResult is the full set of explain rows, sorted by key.
type actionResult struct {
	Entries []entry
}

// -------------------------------------------------------------------------------------
// actionConfig is the explain action's composed config: the shared resolution
// input plus the --reveal toggle and the --output format.
type actionConfig struct {
	config.Input
	Reveal bool
	Output string
}

// -------------------------------------------------------------------------------------
// execute is the imperative shell: resolve the input into an engine.Config, build
// the merged environment, then hand the result to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	ec, err := config.Resolve(&c.Input, p.Project)
	if err != nil {
		return actionResult{}, err
	}
	env, err := engine.Build(ec)
	if err != nil {
		return actionResult{}, err
	}
	return runAction(env, p, c.Reveal)
}

// -------------------------------------------------------------------------------------
// runAction is the pure core: it reads the resolved environment and origins and
// builds sorted rows. Values are masked unless reveal is set. A specific key
// that does not exist is an error.
func runAction(
	env *engine.Result, p actionParams, reveal bool,
) (actionResult, error) {
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

	entries := make([]entry, 0, len(keys))
	for _, key := range keys {
		value, _ := env.Get(key)
		if !reveal {
			value = mask(value)
		}
		origin, _ := env.Origin(key)

		shadowed := make([]string, 0, len(origin.Shadowed))
		for _, s := range origin.Shadowed {
			shadowed = append(shadowed, s.File)
		}

		entries = append(entries, entry{
			Key:       key,
			Value:     value,
			Source:    origin.Winner.File,
			SourceKey: origin.Winner.Key,
			Shadowed:  shadowed,
		})
	}
	return actionResult{Entries: entries}, nil
}

// redacted is the placeholder shown for masked (non-revealed) values.
const redacted = "********"

// -------------------------------------------------------------------------------------
// mask replaces a non-empty value with a fixed redaction marker so secrets are
// not surfaced by default.
func mask(s string) string {
	if s == "" {
		return ""
	}
	return redacted
}
