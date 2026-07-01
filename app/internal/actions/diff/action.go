package diff

import (
	"sort"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
)

// -------------------------------------------------------------------------------------

// actionParams are the positional inputs to the diff action.
type actionParams struct {
	// Project is the project name to resolve under both environments.
	Project string
	// EnvA is the first ("before") environment to resolve.
	EnvA string
	// EnvB is the second ("after") environment to resolve.
	EnvB string
}

// -------------------------------------------------------------------------------------

// actionResult is the data the diff action returns.
type actionResult struct {
	// Added lists keys present only in env-b.
	Added []actionResultChange
	// Removed lists keys present only in env-a.
	Removed []actionResultChange
	// Changed lists keys present in both with differing values.
	Changed []actionResultChange
}

// -------------------------------------------------------------------------------------

// actionResultChange is one row of diff output.
type actionResultChange struct {
	// Key is the env-var key that differs.
	Key string
	// EnvA is the value under env-a (empty for additions).
	EnvA string
	// EnvB is the value under env-b (empty for removals).
	EnvB string
}

// -------------------------------------------------------------------------------------

// execute is the imperative shell: resolve the input into a single envmerge.Params,
// build the merged environment for each specified environment, and hand both results
// to the pure core.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// resolve the shared config
	resolved, err := config.Resolve(in, p.Project)
	if err != nil {
		return actionResult{}, err
	}

	// build each side's environment
	a, err := buildEnv(resolved.Envmerge, p.EnvA)
	if err != nil {
		return actionResult{}, err
	}
	b, err := buildEnv(resolved.Envmerge, p.EnvB)
	if err != nil {
		return actionResult{}, err
	}

	// compare the two environments
	return runAction(a, b), nil
}

// -------------------------------------------------------------------------------------

// buildEnv overrides only the Env on its own copy of the resolved config and
// builds the environment for one diff side. Taking Params by value leaves the
// shared config un-mutated so both sides resolve from identical settings except
// the environment.
func buildEnv(ec envmerge.Params, env string) (*envmerge.Result, error) {
	ec.Settings.Env = env
	return envmerge.Build(ec)
}

// -------------------------------------------------------------------------------------

// runAction is the pure core: a set comparison of two resolved environments.
// Plain data in, structured diff out.
func runAction(a, b *envmerge.Result) actionResult {
	// collect both environments' values
	aAll := a.All()
	bAll := b.All()

	// classify each key as added, removed, or changed
	var res actionResult
	for _, key := range unionKeys(aAll, bAll) {
		av, aok := aAll[key]
		bv, bok := bAll[key]
		switch {
		case aok && !bok:
			res.Removed = append(res.Removed, actionResultChange{Key: key, EnvA: av})
		case !aok && bok:
			res.Added = append(res.Added, actionResultChange{Key: key, EnvB: bv})
		case av != bv:
			res.Changed = append(res.Changed, actionResultChange{Key: key, EnvA: av, EnvB: bv})
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
