package env

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
)

// GetParams selects one key, environment, and reveal policy for a single lookup.
type GetParams struct {
	// Project selects the declared project to resolve.
	Project string
	// Key is the env-var key to look up; it is normalized to uppercase.
	Key string
	// Environment overrides the configured default; an empty value uses it.
	Environment string
	// Reveal controls whether the opened resolver decrypts references.
	Reveal bool
	// Options provides optional resolution setting overrides.
	Options Options
	// Settings provides direct resolution setting values.
	Settings Settings
}

// Get loads the requested environment, selects the single winning value for the
// normalized key, and resolves and renders only that leaf under the call's reveal
// policy. When revealing, it substitutes the requested key's transitive {{ }}
// dependency closure over the effective environment, so a dangling or
// undecryptable value behind an unrelated key cannot block the read while one
// behind a referenced key does. Namespace and flatten failures are fatal, while
// the requested key's own resolution, list-render, missing-reference, or cycle
// failure is returned without leaking its value. A masked get still invokes the
// resolver so implicit references are canonicalized and escaped references are
// unescaped, but it never substitutes: the {{ }} template is shown as declared.
func (s *Service) Get(params GetParams) (GetResult, error) {
	var envOpt *string
	if params.Environment != "" {
		envOpt = &params.Environment
	}
	ctx, err := s.resolveContext(
		params.Project, envOpt, params.Options, params.Settings,
	)
	if err != nil {
		return GetResult{}, err
	}

	namespaces, err := s.loadNamespaces(ctx.includes)
	if err != nil {
		return GetResult{}, err
	}

	state, err := s.mergeLoaded(namespaces, ctx.environment, ctx.settings)
	if err != nil {
		return GetResult{}, err
	}

	// Apply OS source selection over the namespace keys, without unioning OS-only
	// keys, so a get reflects an OS override exactly as run would.
	s.applyOSEnvironment(state, false, ctx.settings.Overload)

	key := strings.ToUpper(params.Key)
	value, ok := state.values[key]
	if !ok {
		return GetResult{}, fmt.Errorf("key %q not found", key)
	}
	origin := state.origins[key]

	resolver, err := s.openResolver(params.Reveal)
	if err != nil {
		return GetResult{}, err
	}

	rendered, err := s.getValue(
		params.Reveal, state, resolver, ctx.environment, key, value, origin,
		ctx.settings, ctx.grammar,
	)
	if err != nil {
		return GetResult{}, err
	}

	return GetResult{
		Key:    key,
		Value:  rendered,
		Source: origin.Winner.File,
		Origin: origin,
	}, nil
}

// getValue renders the requested key's value under the call's reveal policy. A
// masked read resolves and renders only the requested leaf, leaving any {{ }}
// template literal; a revealed read composes the key's transitive dependency
// closure through the substitution engine.
func (s *Service) getValue(
	reveal bool,
	state *mergeState,
	resolver ValueResolver,
	environment, key string,
	value leafValue,
	origin Origin,
	settings Settings,
	grammar *syntax.Grammar,
) (string, error) {
	if reveal {
		engine := s.newSubstituterWith(
			s.getSymbols(state, resolver, environment, settings.Delimiter),
			grammar,
			settings.Overload,
		)
		return engine.Resolve(key)
	}

	resolved, err := resolveLeaf(value, resolver, environment)
	if err != nil {
		return "", err
	}
	return renderLeafValue(resolved, origin.Winner.Key, settings.Delimiter)
}
