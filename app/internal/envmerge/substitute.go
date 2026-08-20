package envmerge

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Substitution grammar delimiters. A value embeds a reference as {{VAR}} for an
// internal namespace variable or {{@VAR}} for an effective (OS-aware) variable; a
// leading backslash on the opening brace makes the whole token a literal.
const (
	refOpen  = "{{"
	refClose = "}}"
	osSigil  = "@"
	escape   = '\\'
)

// tokenKind classifies a span produced by the tokenizer.
type tokenKind int

const (
	// tokenLiteral is verbatim text carried through untouched.
	tokenLiteral tokenKind = iota
	// tokenInternalRef is a {{VAR}} reference to a namespace variable.
	tokenInternalRef
	// tokenOSRef is a {{@VAR}} reference to an effective (OS-aware) variable.
	tokenOSRef
)

// token is one span of a scanned value: literal text, or a reference whose text
// is the trimmed variable name.
type token struct {
	// kind classifies the span.
	kind tokenKind
	// text is literal content for a literal span, or the variable name for a
	// reference span.
	text string
}

// tokenize scans a value into literal spans and reference tokens, honoring the
// \{{ escape that renders the following token literally. An unterminated {{ is
// treated as literal text, so a value can never fail to tokenize.
func tokenize(value string) []token {
	var tokens []token
	var lit strings.Builder

	flush := func() {
		if lit.Len() > 0 {
			tokens = append(tokens, token{kind: tokenLiteral, text: lit.String()})
			lit.Reset()
		}
	}

	for i := 0; i < len(value); {
		// A backslash before an opening brace escapes the token: emit a literal
		// {{ and drop the backslash.
		if value[i] == escape && strings.HasPrefix(value[i+1:], refOpen) {
			lit.WriteString(refOpen)
			i += 1 + len(refOpen)
			continue
		}

		if strings.HasPrefix(value[i:], refOpen) {
			rest := value[i+len(refOpen):]
			end := strings.Index(rest, refClose)
			if end < 0 {
				// No closing brace; treat the opener as literal text.
				lit.WriteString(refOpen)
				i += len(refOpen)
				continue
			}

			flush()
			name := strings.TrimSpace(rest[:end])
			if after, ok := strings.CutPrefix(name, osSigil); ok {
				tokens = append(tokens, token{
					kind: tokenOSRef,
					text: strings.TrimSpace(after),
				})
			} else {
				tokens = append(tokens, token{kind: tokenInternalRef, text: name})
			}
			i += len(refOpen) + end + len(refClose)
			continue
		}

		lit.WriteByte(value[i])
		i++
	}

	flush()
	return tokens
}

// substitutionStatus classifies whether a variable resolves, without exposing its
// composed value.
type substitutionStatus int

const (
	// statusOK marks a variable that composes successfully.
	statusOK substitutionStatus = iota
	// statusUnresolved marks a missing internal or OS reference.
	statusUnresolved
	// statusCircular marks a reference cycle.
	statusCircular
)

// missingInternalReferenceError reports a {{VAR}} that names no namespace
// variable. It carries only key names, never a value.
type missingInternalReferenceError struct {
	// key is the variable whose value holds the dangling reference.
	key string
	// reference is the undefined variable name.
	reference string
}

// Error describes the dangling internal reference without exposing any value.
func (e *missingInternalReferenceError) Error() string {
	return fmt.Sprintf(
		"variable %q references undefined variable %q",
		e.key, e.reference,
	)
}

// missingOSReferenceError reports a {{@VAR}} that resolves in neither the OS
// environment nor the namespace. It carries only names, never a value.
type missingOSReferenceError struct {
	// key is the variable whose value holds the dangling reference.
	key string
	// reference is the variable name that resolves nowhere.
	reference string
}

// Error describes the dangling OS reference without exposing any value.
func (e *missingOSReferenceError) Error() string {
	return fmt.Sprintf(
		"variable %q references %q, which is set in neither the environment "+
			"nor the namespace",
		e.key, e.reference,
	)
}

// circularReferenceError reports a reference cycle as an ordered path of variable
// names. It carries only names, never a value.
type circularReferenceError struct {
	// cycle is the ordered path of variable names, ending where it began.
	cycle []string
}

// Error describes the cycle as a name path without exposing any value.
func (e *circularReferenceError) Error() string {
	return "circular reference: " + strings.Join(e.cycle, " -> ")
}

// symbolTable is the substitution engine's view of the variable namespace. It
// separates a cheap declared check from value resolution so an OS-first {{@VAR}}
// lookup never forces resolution of a namespace value the OS environment
// supersedes, and it marks opaque OS values that must not be re-tokenized.
type symbolTable struct {
	// declared reports whether name is a namespace variable.
	declared func(name string) bool
	// opaque reports whether name's value is an opaque OS value that must be
	// carried through without substitution.
	opaque func(name string) bool
	// value resolves name to its unsubstituted value, decrypting secrets under the
	// active reveal policy. The engine invokes it at most once per name.
	value func(name string) (string, error)
}

// mapSymbols builds a symbolTable over a fully resolved value map keyed the same
// as its origins. Every value is already materialized, so value never fails;
// opacity is read from provenance so an OS-sourced value is carried through
// without substitution.
func mapSymbols(values map[string]string, origins map[string]Origin) symbolTable {
	return symbolTable{
		declared: func(name string) bool { _, ok := values[name]; return ok },
		opaque:   func(name string) bool { return origins[name].Winner.File == osSource },
		value:    func(name string) (string, error) { return values[name], nil },
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

// substituter composes variable references over an effective symbol table. It is
// a string-in/string-out core: the symbol table supplies internal variables and
// their opacity, the getenv seam supplies OS variables, and overload orders the
// {{@VAR}} fallback. A resolved-value cache makes each variable compose once
// regardless of fan-in, and a visiting stack detects cycles.
type substituter struct {
	// symbols is the engine's view of the namespace.
	symbols symbolTable
	// getenv reads an OS variable, reporting whether it is set.
	getenv func(name string) (string, bool)
	// overload flips {{@VAR}} ordering: namespace-then-OS when true, OS-then-
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

// newSubstituter builds an engine over a fully resolved value map, the injected
// getenv seam, and the overload ordering. It backs the pure, table-driven core
// used where every value is already materialized and no value is opaque.
func newSubstituter(
	table map[string]string,
	getenv func(name string) (string, bool),
	overload bool,
) *substituter {
	symbols := symbolTable{
		declared: func(name string) bool { _, ok := table[name]; return ok },
		opaque:   func(string) bool { return false },
		value:    func(name string) (string, error) { return table[name], nil },
	}
	return newSymbolSubstituter(symbols, getenv, overload)
}

// newSymbolSubstituter builds an engine over an arbitrary symbol table, so a
// caller can supply lazy, reveal-gated resolution and opacity.
func newSymbolSubstituter(
	symbols symbolTable,
	getenv func(name string) (string, bool),
	overload bool,
) *substituter {
	return &substituter{
		symbols:  symbols,
		getenv:   getenv,
		overload: overload,
		cache:    make(map[string]string),
		visiting: make(map[string]bool),
		raw:      make(map[string]rawEntry),
	}
}

// rawValue returns a symbol's unsubstituted value, materializing it once and
// memoizing the result so a symbol referenced many times resolves a single time.
func (s *substituter) rawValue(key string) (string, error) {
	if entry, ok := s.raw[key]; ok {
		return entry.value, entry.err
	}
	value, err := s.symbols.value(key)
	s.raw[key] = rawEntry{value: value, err: err}
	return value, err
}

// resolve returns the fully composed value of key, substituting every reference
// transitively. An opaque OS value is returned untouched. A missing reference, a
// cycle, or an underlying resolution failure is returned as a typed error that
// never includes a value.
func (s *substituter) resolve(key string) (string, error) {
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
	if s.symbols.opaque(key) {
		s.cache[key] = raw
		return raw, nil
	}

	s.visiting[key] = true
	s.stack = append(s.stack, key)

	value, err := s.compose(raw, key)

	s.stack = s.stack[:len(s.stack)-1]
	delete(s.visiting, key)
	if err != nil {
		return "", err
	}

	s.cache[key] = value
	return value, nil
}

// status classifies whether key resolves, discarding the composed value so a
// dry-run caller never exposes it.
func (s *substituter) status(key string) substitutionStatus {
	if _, err := s.resolve(key); err != nil {
		var cyclic *circularReferenceError
		if errors.As(err, &cyclic) {
			return statusCircular
		}
		return statusUnresolved
	}
	return statusOK
}

// compose tokenizes value and substitutes each reference, attributing any missing
// reference to key.
func (s *substituter) compose(value, key string) (string, error) {
	var b strings.Builder
	for _, tok := range tokenize(value) {
		switch tok.kind {
		case tokenLiteral:
			b.WriteString(tok.text)
		case tokenInternalRef:
			resolved, err := s.internal(tok.text, key)
			if err != nil {
				return "", err
			}
			b.WriteString(resolved)
		case tokenOSRef:
			resolved, err := s.effective(tok.text, key)
			if err != nil {
				return "", err
			}
			b.WriteString(resolved)
		}
	}
	return b.String(), nil
}

// internal resolves a {{VAR}} reference against the namespace table, re-entering
// the dependency graph so a referenced variable composes transitively.
func (s *substituter) internal(name, key string) (string, error) {
	if !s.symbols.declared(name) {
		return "", &missingInternalReferenceError{key: key, reference: name}
	}
	return s.resolve(name)
}

// effective resolves a {{@VAR}} reference. Without overload the OS environment
// wins and falls back to the namespace; under overload the namespace wins and
// falls back to the OS environment. An OS value is an opaque leaf, while a
// namespace hit re-enters the graph. The namespace value is materialized only
// when it is actually chosen, so a superseded dangling reference never errors.
func (s *substituter) effective(name, key string) (string, error) {
	declared := s.symbols.declared(name)

	if s.overload {
		if declared {
			return s.resolve(name)
		}
		if value, ok := s.getenv(name); ok {
			return value, nil
		}
		return "", &missingOSReferenceError{key: key, reference: name}
	}

	if value, ok := s.getenv(name); ok {
		return value, nil
	}
	if declared {
		return s.resolve(name)
	}
	return "", &missingOSReferenceError{key: key, reference: name}
}

// cycleError builds a circular-reference error from the active resolution path,
// starting at the earlier visit of key and closing the loop back to it.
func (s *substituter) cycleError(key string) error {
	start := slices.Index(s.stack, key)
	cycle := append(slices.Clone(s.stack[start:]), key)
	return &circularReferenceError{cycle: cycle}
}
