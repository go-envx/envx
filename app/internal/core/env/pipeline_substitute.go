package env

import (
	"errors"
	"slices"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/env/syntax"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// symbolTable is the substitution engine's view of the namespace.
type symbolTable struct {
	// declared reports whether name is a namespace variable.
	declared func(name string) bool
	// opaque reports whether name's value must not be substituted.
	opaque func(name string) bool
	// value returns name's unsubstituted value; called at most once per name.
	value func(name string) (string, error)
}

// substituter composes {{ }} references over a symbol table and the host
// environment, memoizing each composed variable and detecting cycles.
type substituter struct {
	// grammar tokenizes values.
	grammar *syntax.Grammar
	// symbols is the namespace view.
	symbols symbolTable
	// host supplies host variables for references.
	host hostenv.Snapshot
	// overload prefers namespace variables over host variables.
	overload bool
	// composed memoizes composed values.
	composed map[string]string
	// raw memoizes unsubstituted values and their errors.
	raw map[string]rawValue
	// stack is the active resolution path for cycle detection.
	stack []string
}

// rawValue memoizes one symbol's unsubstituted value.
type rawValue struct {
	// text is the unsubstituted value.
	text string
	// err is the symbol's resolution error.
	err error
}

// newSubstituter builds an engine for one operation.
func newSubstituter(op operation, symbols symbolTable) *substituter {
	return &substituter{
		grammar:  op.grammar,
		symbols:  symbols,
		host:     op.host,
		overload: op.settings.overload,
		composed: make(map[string]string),
		raw:      make(map[string]rawValue),
	}
}

// resolve returns key's fully composed value. Opaque values are returned
// untouched. Missing references and cycles are typed errors without values.
func (s *substituter) resolve(key string) (string, error) {
	if v, ok := s.composed[key]; ok {
		return v, nil
	}
	if slices.Contains(s.stack, key) {
		start := slices.Index(s.stack, key)
		cycle := append(slices.Clone(s.stack[start:]), key)
		return "", &CircularReferenceError{Cycle: cycle}
	}

	raw, ok := s.raw[key]
	if !ok {
		text, err := s.symbols.value(key)
		raw = rawValue{text: text, err: err}
		s.raw[key] = raw
	}
	if raw.err != nil {
		return "", raw.err
	}
	if s.symbols.opaque(key) {
		s.composed[key] = raw.text
		return raw.text, nil
	}

	s.stack = append(s.stack, key)
	composed, err := s.grammar.Substitute(raw.text, func(name string) (string, error) {
		return s.reference(name, key)
	})
	s.stack = s.stack[:len(s.stack)-1]
	if err != nil {
		return "", err
	}
	s.composed[key] = composed
	return composed, nil
}

// reference resolves one reference: host first by default, namespace first
// under overload. A namespace value is resolved only when chosen.
func (s *substituter) reference(name, key string) (string, error) {
	declared := s.symbols.declared(name)
	hostValue, inHost := s.host.Get(name)
	switch {
	case s.overload && declared:
		return s.resolve(name)
	case inHost:
		return hostValue, nil
	case declared:
		return s.resolve(name)
	default:
		return "", &MissingReferenceError{Key: key, Reference: name}
	}
}

// substitutionCode classifies a substitution failure.
func substitutionCode(err error) diagnostic.Code {
	if errors.Is(err, ErrCircularReference) {
		return diagnostic.CircularVariableReference
	}
	return diagnostic.UnresolvedVariableReference
}
