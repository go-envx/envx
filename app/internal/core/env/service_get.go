package env

import (
	"fmt"
	"strings"
)

// GetParams selects one key and the reveal policy.
type GetParams struct {
	// Query selects the project and setting overrides.
	Query Query
	// Key is the case-insensitive variable name.
	Key string
	// Reveal decrypts secrets and substitutes references.
	Reveal bool
}

// GetResult is one resolved value and its provenance.
type GetResult struct {
	// Key is the canonical variable name.
	Key string
	// Value is the rendered value under the reveal policy.
	Value string
	// Origin records the winning and shadowed sources.
	Origin Origin
}

// Get resolves one key. Masked, the leaf is resolved without substitution so
// references are canonicalized but templates stay as declared. Revealed, only
// the key's transitive dependencies are resolved, so failures behind unrelated
// keys never block the read. Errors never contain the value.
func (s *Service) Get(params GetParams) (GetResult, error) {
	op, err := s.begin(params.Query)
	if err != nil {
		return GetResult{}, err
	}
	merged, err := s.load(op, false)
	if err != nil {
		return GetResult{}, err
	}

	key := strings.ToUpper(params.Key)
	leaf, ok := merged.values[key]
	if !ok {
		return GetResult{}, fmt.Errorf("%w: %q", ErrKeyNotFound, key)
	}
	origin := merged.origins[key]

	resolver, err := s.newValueResolver(params.Reveal, op.host)
	if err != nil {
		return GetResult{}, err
	}

	var rendered string
	if params.Reveal {
		rendered, err = newSubstituter(op, merged.symbols(resolver, op)).resolve(key)
	} else {
		var resolved leafValue
		if resolved, err = resolveLeaf(leaf, resolver, op.environment); err == nil {
			rendered, err = renderLeaf(resolved, origin.Winner.Key, op.settings.delimiter)
		}
	}
	if err != nil {
		return GetResult{}, err
	}
	return GetResult{Key: key, Value: rendered, Origin: origin}, nil
}
