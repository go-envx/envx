package get

import (
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

// execute is the imperative shell: resolve the project configuration, construct a
// manager, and look up the single requested key. Secret references are masked
// unless p.Reveal is set.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// resolve the input config
	resolved, err := config.ResolveProject(in, p.Project, p.Reveal)
	if err != nil {
		return actionResult{}, err
	}

	// construct the manager from the resolved params
	manager, err := envmerge.New(resolved.Envmerge)
	if err != nil {
		return actionResult{}, err
	}

	// resolve only the requested key; the environment comes from the
	// precedence-resolved default the manager already carries.
	entry, err := manager.Get(envmerge.GetParams{
		Key:    p.Key,
		Reveal: p.Reveal,
	})
	if err != nil {
		return actionResult{}, err
	}

	return actionResult{
		Value:  entry.Value,
		Source: entry.Origin.Winner.File,
	}, nil
}
