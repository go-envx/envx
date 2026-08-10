package envmerge

import (
	"maps"
	"sort"
)

// resolved holds the merged, flattened env-var values and their per-key
// origins. It is the mutable working state assembled during a merge; Result
// embeds it to expose a read-only view once resolution is complete.
type resolved struct {
	// values holds the merged, flattened env-var key/value pairs.
	values map[string]string
	// origins records, per key, the winning source and any it shadowed.
	origins map[string]Origin
}

// Result is the immutable outcome of resolution. Its maps are unexported;
// consumers read them through the methods below, which return copies or sorted
// views so the internals stay un-mutated.
type Result struct {
	resolved
}

// Source records where a resolved value came from: the absolute path to the
// YAML file and the original nested key before flattening (e.g.
// "postgres.password").
type Source struct {
	// File is the absolute path to the YAML file the value came from.
	File string
	// Key is the original nested key before flattening (e.g. "postgres.password").
	Key string
}

// Origin describes where a resolved value came from, plus every source it
// shadowed during the merge.
type Origin struct {
	// Winner is the source whose value survived the merge.
	Winner Source
	// Shadowed lists the sources the winner overrode, in merge order.
	Shadowed []Source
}

// Get returns the value of a single key and whether it was present.
func (r *Result) Get(key string) (string, bool) {
	v, ok := r.values[key]
	return v, ok
}

// All returns a copy of the full resolved environment, safe for the caller to
// mutate.
func (r *Result) All() map[string]string {
	out := make(map[string]string, len(r.values))
	maps.Copy(out, r.values)
	return out
}

// Origin returns the resolution history for a key and whether it was present.
func (r *Result) Origin(key string) (Origin, bool) {
	o, ok := r.origins[key]
	return o, ok
}

// Keys returns the resolved keys in sorted order.
func (r *Result) Keys() []string {
	keys := make([]string, 0, len(r.values))
	for k := range r.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
