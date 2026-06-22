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
	Project string
	EnvA    string
	EnvB    string
}

// -------------------------------------------------------------------------------------
// change records a single difference between the two environments. EnvA is empty
// for additions and EnvB is empty for removals.
type change struct {
	Key  string
	EnvA string
	EnvB string
}

// -------------------------------------------------------------------------------------
// actionResult is the structured diff: keys only in env-b, only in env-a,
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
// the environment under each positional environment via buildEngine and feed
// both results to the pure core.
func execute(p actionParams, c *actionConfig) (actionResult, error) {
	ec, err := config.Resolve(&c.Input, p.Project)
	if err != nil {
		return actionResult{}, err
	}

	a, err := buildEngine(ec, p.EnvA)
	if err != nil {
		return actionResult{}, err
	}
	b, err := buildEngine(ec, p.EnvB)
	if err != nil {
		return actionResult{}, err
	}
	return runAction(a, b), nil
}

// -------------------------------------------------------------------------------------
// buildEngine copies the resolved config, overrides only its Env, and builds the
// environment for one diff side. Copying leaves the shared config un-mutated so
// both sides resolve from identical settings except the environment.
func buildEngine(ec *engine.Config, env string) (*engine.Result, error) {
	cfg := *ec
	cfg.Settings.Env = env
	return engine.Build(&cfg)
}

// -------------------------------------------------------------------------------------
// runAction is the pure core: a set comparison of two resolved environments.
// Plain data in, structured diff out.
func runAction(a, b *engine.Result) actionResult {
	aAll := a.All()
	bAll := b.All()

	var res actionResult
	for _, key := range unionKeys(aAll, bAll) {
		av, aok := aAll[key]
		bv, bok := bAll[key]
		switch {
		case aok && !bok:
			res.Removed = append(res.Removed, change{Key: key, EnvA: av})
		case !aok && bok:
			res.Added = append(res.Added, change{Key: key, EnvB: bv})
		case av != bv:
			res.Changed = append(res.Changed, change{Key: key, EnvA: av, EnvB: bv})
		}
	}
	return res
}

// -------------------------------------------------------------------------------------
// unionKeys returns the sorted union of the keys of two maps.
func unionKeys(a, b map[string]string) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
