package envmerge

import (
	"fmt"
	"strings"
)

// Entry is one successfully resolved winning env-var value and its provenance.
type Entry struct {
	// Key is the canonical uppercase env-var key that was looked up.
	Key string
	// Value is the resolved, rendered value of the winning leaf.
	Value string
	// Origin records the winning source and every source it shadowed.
	Origin Origin
}

// GetParams selects one key, environment, and reveal policy for a single lookup.
type GetParams struct {
	// Key is the env-var key to look up; it is normalized to uppercase.
	Key string
	// Environment overrides the configured default; an empty value uses it.
	Environment string
	// Reveal controls whether the opened resolver decrypts references.
	Reveal bool
}

// Get loads the requested environment, selects the single winning value for the
// normalized key, and resolves and renders only that leaf under the call's reveal
// policy. Unrelated references never reach the resolver, so a dangling or
// undecryptable value behind a different key cannot block the read. Namespace and
// flatten failures are fatal, while the requested key's own resolution or
// list-render failure is returned without leaking its value. A masked get still
// invokes the resolver so implicit references are canonicalized and escaped
// references are unescaped; it is not a raw literal read.
func (m *Manager) Get(params GetParams) (Entry, error) {
	environment, err := m.normalizeEnvironment(params.Environment)
	if err != nil {
		return Entry{}, err
	}

	state, err := m.merge(environment)
	if err != nil {
		return Entry{}, err
	}

	key := strings.ToUpper(params.Key)
	value, ok := state.values[key]
	if !ok {
		return Entry{}, fmt.Errorf("key %q not found", key)
	}
	origin := state.origins[key]

	resolver, err := m.openResolver(params.Reveal)
	if err != nil {
		return Entry{}, err
	}

	resolved, err := resolveLeaf(value, resolver, environment)
	if err != nil {
		return Entry{}, err
	}
	rendered, err := renderLeafValue(
		resolved, origin.Winner.Key, m.params.Settings.Delimiter,
	)
	if err != nil {
		return Entry{}, err
	}

	return Entry{Key: key, Value: rendered, Origin: origin}, nil
}
