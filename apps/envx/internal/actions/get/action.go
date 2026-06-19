package get

import (
	"fmt"
	"strings"

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
// runAction is the pure core: a case-insensitive lookup against an already
// resolved environment. Plain data in, plain data out — no engine call, no I/O,
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
