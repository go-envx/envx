package syntax

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Resolution classifies whether a variable resolves, without exposing its
// composed value.
type Resolution int

const (
	// ResolutionOK marks a variable that composes successfully.
	ResolutionOK Resolution = iota
	// ResolutionUnresolved marks a reference that resolves nowhere.
	ResolutionUnresolved
	// ResolutionCircular marks a reference cycle.
	ResolutionCircular
)

// String returns a human-readable name for the resolution status.
func (r Resolution) String() string {
	switch r {
	case ResolutionOK:
		return "ok"
	case ResolutionUnresolved:
		return "unresolved"
	case ResolutionCircular:
		return "circular"
	default:
		return fmt.Sprintf("Resolution(%d)", int(r))
	}
}

// SymbolTable provides the substitution engine's view of the variable namespace.
// It separates a declared check from value resolution so an OS-first reference
// lookup never forces resolution of a namespace value the OS environment
// supersedes, and it marks opaque OS values that must not be re-tokenized.
type SymbolTable struct {
	// Declared reports whether name is a namespace variable.
	Declared func(name string) bool
	// Opaque reports whether name's value is an opaque value that must be
	// carried through without substitution.
	Opaque func(name string) bool
	// Value resolves name to its unsubstituted value. The engine invokes it at
	// most once per name.
	Value func(name string) (string, error)
}

// MapSymbols builds a SymbolTable over a plain map where no value is opaque.
func MapSymbols(table map[string]string) SymbolTable {
	return SymbolTable{
		Declared: func(name string) bool { _, ok := table[name]; return ok },
		Opaque:   func(string) bool { return false },
		Value:    func(name string) (string, error) { return table[name], nil },
	}
}

// rawEntry memoizes one symbol's unsubstituted value and any resolution error so
// each symbol is materialized at most once regardless of fan-in.
type rawEntry struct {
	// value is the symbol's unsubstituted value.
	value string
	// err is the resolution error, if any.
	err error
}

// SubstituterParams specifies parameters for constructing a Substituter.
type SubstituterParams struct {
	// Grammar is the compiled reference syntax the engine tokenizes values with.
	// When nil, DefaultGrammar is used.
	Grammar *Grammar
	// Symbols is the engine's view of the namespace.
	Symbols SymbolTable
	// Getenv reads an OS variable, reporting whether it is set. When nil, no OS
	// variables are resolved.
	Getenv func(name string) (string, bool)
	// Overload flips reference ordering: namespace-then-OS when true, OS-then-
	// namespace when false.
	Overload bool
}

// Substituter composes variable references over an effective symbol table. It is
// a string-in/string-out core: the symbol table supplies namespace variables and
// their opacity, the getenv seam supplies OS variables, and overload orders the
// namespace/OS fallback. A resolved-value cache makes each variable compose once
// regardless of fan-in, and a visiting stack detects cycles.
type Substituter struct {
	// grammar is the compiled reference syntax the engine tokenizes values with.
	grammar *Grammar
	// symbols is the engine's view of the namespace.
	symbols SymbolTable
	// getenv reads an OS variable, reporting whether it is set.
	getenv func(name string) (string, bool)
	// overload flips reference ordering: namespace-then-OS when true, OS-then-
	// namespace when false.
	overload bool
	// cache memoizes each variable's composed value.
	cache map[string]string
	// visiting marks variables on the active resolution path for cycle detection.
	visiting map[string]bool
	// stack records the active resolution path to report a cycle.
	stack []string
	// raw memoizes each symbol's unsubstituted value and resolution error.
	raw map[string]rawEntry
}

// NewSubstituter builds an engine over the provided parameters.
func NewSubstituter(params SubstituterParams) *Substituter {
	grammar := params.Grammar
	if grammar == nil {
		grammar = DefaultGrammar
	}
	getenv := params.Getenv
	if getenv == nil {
		getenv = func(string) (string, bool) { return "", false }
	}
	symbols := params.Symbols
	if symbols.Declared == nil {
		symbols.Declared = func(string) bool { return false }
	}
	if symbols.Opaque == nil {
		symbols.Opaque = func(string) bool { return false }
	}
	if symbols.Value == nil {
		symbols.Value = func(string) (string, error) { return "", nil }
	}
	return &Substituter{
		grammar:  grammar,
		symbols:  symbols,
		getenv:   getenv,
		overload: params.Overload,
		cache:    make(map[string]string),
		visiting: make(map[string]bool),
		raw:      make(map[string]rawEntry),
	}
}

// Grammar returns the compiled reference syntax used by this substituter.
func (s *Substituter) Grammar() *Grammar {
	return s.grammar
}

// rawValue returns a symbol's unsubstituted value, materializing it once and
// memoizing the result so a symbol referenced many times resolves a single time.
func (s *Substituter) rawValue(key string) (string, error) {
	if entry, ok := s.raw[key]; ok {
		return entry.value, entry.err
	}
	value, err := s.symbols.Value(key)
	s.raw[key] = rawEntry{value: value, err: err}
	return value, err
}

// Resolve returns the fully composed value of key, substituting every reference
// transitively. An opaque value is returned untouched. A missing reference, a
// cycle, or an underlying resolution failure is returned as a typed error that
// never includes a value.
func (s *Substituter) Resolve(key string) (string, error) {
	if value, ok := s.cache[key]; ok {
		return value, nil
	}
	if s.visiting[key] {
		return "", s.cycleError(key)
	}

	raw, err := s.rawValue(key)
	if err != nil {
		return "", err
	}
	if s.symbols.Opaque(key) {
		s.cache[key] = raw
		return raw, nil
	}

	s.visiting[key] = true
	s.stack = append(s.stack, key)

	value, err := s.Compose(raw, key)

	s.stack = s.stack[:len(s.stack)-1]
	delete(s.visiting, key)
	if err != nil {
		return "", err
	}

	s.cache[key] = value
	return value, nil
}

// Status classifies whether key resolves, discarding the composed value so a
// dry-run caller never exposes it.
func (s *Substituter) Status(key string) Resolution {
	if _, err := s.Resolve(key); err != nil {
		var cyclic *CircularReferenceError
		if errors.As(err, &cyclic) {
			return ResolutionCircular
		}
		return ResolutionUnresolved
	}
	return ResolutionOK
}

// Compose tokenizes value and substitutes each reference, attributing any missing
// reference to key.
func (s *Substituter) Compose(value, key string) (string, error) {
	var b strings.Builder
	for _, tok := range s.grammar.Tokenize(value) {
		switch tok.Kind {
		case TokenLiteral:
			b.WriteString(tok.Text)
		case TokenReference:
			resolved, err := s.reference(tok.Text, key)
			if err != nil {
				return "", err
			}
			b.WriteString(resolved)
		}
	}
	return b.String(), nil
}

// reference resolves a variable reference against the composed environment. Without
// overload the OS environment wins and falls back to the namespace; under overload
// the namespace wins and falls back to the OS environment — the same precedence a
// run child process sees. An OS value is an opaque leaf, while a namespace hit
// re-enters the graph so a referenced variable composes transitively. The
// namespace value is materialized only when it is actually chosen, so a superseded
// dangling reference never errors.
func (s *Substituter) reference(name, key string) (string, error) {
	declared := s.symbols.Declared(name)

	if s.overload {
		if declared {
			return s.Resolve(name)
		}
		if value, ok := s.getenv(name); ok {
			return value, nil
		}
		return "", &MissingReferenceError{Key: key, Reference: name}
	}

	if value, ok := s.getenv(name); ok {
		return value, nil
	}
	if declared {
		return s.Resolve(name)
	}
	return "", &MissingReferenceError{Key: key, Reference: name}
}

// cycleError builds a circular-reference error from the active resolution path,
// starting at the earlier visit of key and closing the loop back to it.
func (s *Substituter) cycleError(key string) error {
	start := slices.Index(s.stack, key)
	cycle := append(slices.Clone(s.stack[start:]), key)
	return &CircularReferenceError{Cycle: cycle}
}
