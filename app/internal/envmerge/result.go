package envmerge

import (
	"errors"
	"fmt"
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
	// errs records, per key, a deferred resolution or render failure so a
	// single-key consumer can ignore failures in unrelated keys.
	errs map[string]error
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

// Err returns the deferred resolution error recorded for key during the merge,
// or nil when the key resolved successfully or is absent. A single-key command
// uses it to surface only its own key's failure while ignoring unrelated
// dangling references.
func (r *Result) Err(key string) error {
	return r.errs[key]
}

// Verify returns every deferred resolution failure, one per key in sorted key
// order and each wrapped with its failing key, or nil when every key resolved.
// Whole-environment consumers call it to fail loudly on any dangling reference
// before exposing a partial environment (for example, before starting a child
// process); reporting every unresolved key at once lets the user fix them
// together instead of one at a time.
func (r *Result) Verify() error {
	if len(r.errs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(r.errs))
	for key := range r.errs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	failures := make([]error, 0, len(keys))
	for _, key := range keys {
		failures = append(failures, fmt.Errorf("resolving %s: %w", key, r.errs[key]))
	}
	return errors.Join(failures...)
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
