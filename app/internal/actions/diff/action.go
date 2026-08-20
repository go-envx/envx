package diff

import (
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
)

// actionParams are the inputs to the diff action.
type actionParams struct {
	// Project is the project name to resolve under both environments.
	Project string
	// EnvA is the first ("before") environment to resolve.
	EnvA string
	// EnvB is the second ("after") environment to resolve.
	EnvB string
	// Reveal controls whether each side is resolved and substituted before the
	// comparison; masked, references are compared as declarations.
	Reveal bool
}

// actionResult is the data the diff action returns.
type actionResult struct {
	// Added lists keys present only in env-b.
	Added []actionResultChange
	// Removed lists keys present only in env-a.
	Removed []actionResultChange
	// Changed lists keys present in both with differing values.
	Changed []actionResultChange
}

// actionResultChange is one row of diff output.
type actionResultChange struct {
	// Key is the env-var key that differs.
	Key string
	// EnvA is the value under env-a (empty for additions).
	EnvA string
	// EnvB is the value under env-b (empty for removals).
	EnvB string
}

// execute is the imperative shell: resolve the project configuration and compare
// the two declared environments through the constructed manager. Masked, diff
// compares declarations and never decrypts; revealed, it resolves and
// substitutes each side before comparing.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// resolve the shared config.
	resolved, err := config.ResolveProject(in, p.Project)
	if err != nil {
		return actionResult{}, err
	}

	// compare the two environments under the requested reveal policy.
	difference, err := resolved.Envmerge.Diff(envmerge.DiffParams{
		EnvironmentA: p.EnvA,
		EnvironmentB: p.EnvB,
		Reveal:       p.Reveal,
	})
	if err != nil {
		return actionResult{}, err
	}

	return buildResult(difference), nil
}

// buildResult maps the envmerge comparison into the action's presentation shape,
// carrying each side's literal value into the stable env-a/env-b naming.
func buildResult(difference *envmerge.DiffResult) actionResult {
	return actionResult{
		Added:   toChanges(difference.Added),
		Removed: toChanges(difference.Removed),
		Changed: toChanges(difference.Changed),
	}
}

// toChanges converts envmerge changes into the action's presentation rows.
func toChanges(in []envmerge.Change) []actionResultChange {
	out := make([]actionResultChange, 0, len(in))
	for _, c := range in {
		out = append(out, actionResultChange{Key: c.Key, EnvA: c.Before, EnvB: c.After})
	}
	return out
}
