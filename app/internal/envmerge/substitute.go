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

// substituter composes variable references over an effective symbol table. It is
// a pure, string-in/string-out core: the namespace table supplies internal
// variables, the getenv seam supplies OS variables, and overload orders the
// {{@VAR}} fallback. A resolved-value cache makes each variable compose once
// regardless of fan-in, and a visiting stack detects cycles.
type substituter struct {
	// table maps each internal variable name to its unresolved value.
	table map[string]string
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
}

// newSubstituter builds an engine over the effective symbol table, the injected
// getenv seam, and the overload ordering.
func newSubstituter(
	table map[string]string,
	getenv func(name string) (string, bool),
	overload bool,
) *substituter {
	return &substituter{
		table:    table,
		getenv:   getenv,
		overload: overload,
		cache:    make(map[string]string),
		visiting: make(map[string]bool),
	}
}

// resolve returns the fully composed value of key, substituting every reference
// transitively. A missing reference or a cycle is returned as a typed error that
// never includes a value.
func (s *substituter) resolve(key string) (string, error) {
	if value, ok := s.cache[key]; ok {
		return value, nil
	}
	if s.visiting[key] {
		return "", s.cycleError(key)
	}

	raw := s.table[key]
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
	if _, ok := s.table[name]; !ok {
		return "", &missingInternalReferenceError{key: key, reference: name}
	}
	return s.resolve(name)
}

// effective resolves a {{@VAR}} reference. Without overload the OS environment
// wins and falls back to the namespace; under overload the namespace wins and
// falls back to the OS environment. An OS value is an opaque leaf, while a
// namespace hit re-enters the graph.
func (s *substituter) effective(name, key string) (string, error) {
	_, inTable := s.table[name]

	if s.overload {
		if inTable {
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
	if inTable {
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
