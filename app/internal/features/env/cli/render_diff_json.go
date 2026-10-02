package cli

import (
	"github.com/go-envx/envx/app/internal/features/env"
)

// diffResultJSON is the exported, tagged view of the whole diff for JSON output.
type diffResultJSON struct {
	Added   []diffChangesJSON `json:"added,omitempty"`
	Removed []diffChangesJSON `json:"removed,omitempty"`
	Changed []diffChangesJSON `json:"changed,omitempty"`
}

// diffChangesJSON is the exported, tagged view of a change used for JSON output.
type diffChangesJSON struct {
	Key  string `json:"key"`
	EnvA string `json:"env_a,omitempty"`
	EnvB string `json:"env_b,omitempty"`
}

// json writes the diff as an indented JSON object.
func (r diffRenderer) json(res *env.DiffResult) error {
	return r.console.WriteJSON(diffResultJSON{
		Added:   r.toDiffChangesJSON(res.Added),
		Removed: r.toDiffChangesJSON(res.Removed),
		Changed: r.toDiffChangesJSON(res.Changed),
	})
}

// toDiffChangesJSON converts env changes to their tagged JSON view.
func (r diffRenderer) toDiffChangesJSON(in []env.Change) []diffChangesJSON {
	out := make([]diffChangesJSON, 0, len(in))
	for _, c := range in {
		out = append(out, diffChangesJSON{
			Key:  c.Key,
			EnvA: c.Before,
			EnvB: c.After,
		})
	}
	return out
}
