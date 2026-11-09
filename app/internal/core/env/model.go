package env

import (
	"maps"
	"slices"

	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/core/workspace"
)

// Query selects a project and the setting overrides for an operation.
type Query struct {
	// Project names the declared project; empty selects the only project.
	Project string
	// Options are flag or environment overrides that win over the manifest.
	Options workspace.Settings
}

// Source identifies one contribution to a value.
type Source struct {
	// Path is the namespace file; empty for a host value.
	Path string
	// Key is the dotted file key, or the variable name for a host value.
	Key string
	// Host marks a value taken from the host environment.
	Host bool
}

// Origin records the winning source and every source it shadowed.
type Origin struct {
	// Winner is the source whose value survived the merge.
	Winner Source
	// Shadowed lists overridden sources, oldest first.
	Shadowed []Source
}

// Entry is one winning variable with its provenance and evaluation.
type Entry struct {
	// Key is the canonical variable name.
	Key string
	// Raw is the winning value as declared, with list items joined; empty for a
	// host value.
	Raw string
	// Items are the raw scalar items before joining; nil for a host value.
	Items []string
	// Evaluation is the value's classification and diagnosis.
	Evaluation value.Evaluation
	// Origin records the winning and shadowed sources.
	Origin Origin
	// Secret reports whether the value derives from a secret, directly or
	// through substitution.
	Secret bool
	// DeclaredInBase reports whether a namespace base file declares the key.
	DeclaredInBase bool
}

// Environment is a complete set of materialized values with provenance. The
// zero Environment is empty.
type Environment struct {
	// values holds the materialized variables.
	values map[string]string
	// origins records each variable's provenance.
	origins map[string]Origin
}

// Get returns one materialized value and whether it exists.
func (e Environment) Get(key string) (string, bool) {
	v, ok := e.values[key]
	return v, ok
}

// All returns a copy of every materialized variable.
func (e Environment) All() map[string]string {
	out := make(map[string]string, len(e.values))
	maps.Copy(out, e.values)
	return out
}

// Origin returns a copy of one variable's provenance and whether it exists.
func (e Environment) Origin(key string) (Origin, bool) {
	o, ok := e.origins[key]
	o.Shadowed = slices.Clone(o.Shadowed)
	return o, ok
}
