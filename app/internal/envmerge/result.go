package envmerge

import (
	"maps"
	"sort"
)

// -------------------------------------------------------------------------------------
// Result is the immutable outcome of resolution. Its maps are unexported;
// consumers read them through the methods below, which return copies or sorted
// views so the internals stay un-mutated.
type Result struct {
	values  map[string]string
	origins map[string]Origin
}

// -------------------------------------------------------------------------------------
// Source records where a resolved value came from: the absolute path to the
// YAML file and the original nested key before flattening (e.g.
// "postgres.password").
type Source struct {
	File string
	Key  string
}

// -------------------------------------------------------------------------------------
// Origin describes where a resolved value came from, plus every source it
// shadowed during the merge. It powers the explain action.
type Origin struct {
	Winner   Source
	Shadowed []Source
}

// -------------------------------------------------------------------------------------
// Get returns the value of a single key and whether it was present.
func (r *Result) Get(key string) (string, bool) {
	v, ok := r.values[key]
	return v, ok
}

// -------------------------------------------------------------------------------------
// All returns a copy of the full resolved environment, safe for the caller to
// mutate.
func (r *Result) All() map[string]string {
	out := make(map[string]string, len(r.values))
	maps.Copy(out, r.values)
	return out
}

// -------------------------------------------------------------------------------------
// Origin returns the resolution history for a key and whether it was present.
func (r *Result) Origin(key string) (Origin, bool) {
	o, ok := r.origins[key]
	return o, ok
}

// -------------------------------------------------------------------------------------
// Keys returns the resolved keys in sorted order.
func (r *Result) Keys() []string {
	keys := make([]string, 0, len(r.values))
	for k := range r.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
