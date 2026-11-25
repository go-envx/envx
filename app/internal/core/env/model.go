package env

import (
	"maps"
	"slices"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/value"
)

// SourceKind classifies where a value came from.
type SourceKind int

// Source kinds. The zero value is a namespace file.
const (
	SourceNamespace SourceKind = iota
	SourceLocalEnv
	SourceProcess
)

// String returns the kind's stable machine-readable name.
func (k SourceKind) String() string {
	switch k {
	case SourceLocalEnv:
		return "localEnv"
	case SourceProcess:
		return "process"
	default:
		return "namespace"
	}
}

// Source identifies one contribution to a value.
type Source struct {
	// Path is the namespace file or the local env file; empty for a process
	// variable.
	Path string
	// Key is the dotted file key, or the variable name for a host value.
	Key string
	// Kind is where the value came from.
	Kind SourceKind
}

// IsHost reports whether the value came from the host environment, whether the
// process or the local env file.
func (s Source) IsHost() bool {
	return s.Kind != SourceNamespace
}

// newHostSource returns the source of a host value read from file, which is
// empty for a process variable.
func newHostSource(key, file string) Source {
	if file == "" {
		return Source{Key: key, Kind: SourceProcess}
	}
	return Source{Path: file, Key: key, Kind: SourceLocalEnv}
}

// Origin records the winning source and every source it shadowed.
type Origin struct {
	// Winner is the source whose value survived the merge.
	Winner Source
	// Shadowed lists overridden sources, oldest first.
	Shadowed []Source
}

// ExplainEntry is one winning variable with its provenance and evaluation.
type ExplainEntry struct {
	// Key is the canonical variable name.
	Key string
	// Raw is the winning value as declared, with list items joined; empty for a
	// host value.
	Raw string
	// Items are the raw namespace items before joining. For a host winner they
	// are the replaced namespace value's items, or nil for a host-only key.
	Items []string
	// Evaluation is the value's classification and diagnosis.
	Evaluation value.Evaluation
	// Findings lists every distinct finding in item order; Evaluation holds the
	// most severe.
	Findings []diagnostic.Finding
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
