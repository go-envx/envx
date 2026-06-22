package diff

import (
	"sort"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
)

// -------------------------------------------------------------------------------------
// actionParams are the inputs to diff: one project resolved under two
// environments.
type actionParams struct {
	Project  string
	LeftEnv  string
	RightEnv string
}

// -------------------------------------------------------------------------------------
// change records a single difference between the two environments. Left is empty
// for additions and Right is empty for removals.
type change struct {
	Key   string
	Left  string
	Right string
}

// -------------------------------------------------------------------------------------
// actionResult is the structured diff: keys only in the right, only in the left,
// and present in both with differing values.
type actionResult struct {
	Added   []change
	Removed []change
	Changed []change
}

// -------------------------------------------------------------------------------------
// actionConfig is the diff action's composed config: the shared resolution input
// (the two environments come from positional args, not --env) plus the --reveal
// toggle and the --output format.
type actionConfig struct {
	config.Input
	Reveal bool
	Output string
}

// -------------------------------------------------------------------------------------
// execute is the imperative shell: resolve the merge settings once, then build
// the environment under each positional environment (overriding Settings.Env per
// side) and feed both results to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	ec, err := config.Resolve(&c.Input, p.Project)
	if err != nil {
		return actionResult{}, err
	}

	ec.Settings.Env = p.LeftEnv
	left, err := engine.Build(ec)
	if err != nil {
		return actionResult{}, err
	}

	ec.Settings.Env = p.RightEnv
	right, err := engine.Build(ec)
	if err != nil {
		return actionResult{}, err
	}
	return runAction(left, right), nil
}

// -------------------------------------------------------------------------------------
// runAction is the pure core: a set comparison of two resolved environments.
// Plain data in, structured diff out.
func runAction(left, right *engine.Result) actionResult {
	leftAll := left.All()
	rightAll := right.All()

	var res actionResult
	for _, key := range unionKeys(leftAll, rightAll) {
		lv, lok := leftAll[key]
		rv, rok := rightAll[key]
		switch {
		case lok && !rok:
			res.Removed = append(res.Removed, change{Key: key, Left: lv})
		case !lok && rok:
			res.Added = append(res.Added, change{Key: key, Right: rv})
		case lv != rv:
			res.Changed = append(res.Changed, change{Key: key, Left: lv, Right: rv})
		}
	}
	return res
}

// -------------------------------------------------------------------------------------
// unionKeys returns the sorted union of the keys of two maps.
func unionKeys(left, right map[string]string) []string {
	set := make(map[string]struct{}, len(left)+len(right))
	for k := range left {
		set[k] = struct{}{}
	}
	for k := range right {
		set[k] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
