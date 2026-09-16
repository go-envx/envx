package envmerge

import (
	"fmt"
	"regexp"
	"strings"
)

// escape makes the following reference token a literal: a backslash immediately
// before a reference match renders the matched text verbatim and drops the
// backslash, so a value can carry an untouched Go/Helm template.
const escape = '\\'

// Default reference grammar. The internal pattern matches {{VAR}} for a namespace
// variable; the OS pattern matches {{@VAR}} for an effective (OS-aware) variable.
// Each pattern's first capture group is the variable name, which the tokenizer
// trims. The internal pattern excludes a leading @ so it never shadows an OS
// reference, and both stop at the first "}}" so a value can hold literal text
// after the reference. A workspace overrides either pattern via the reference-
// pattern settings; the exact sigil is therefore not load-bearing.
const (
	defaultReferencePattern   = `\{\{(\s*[^@][^}]*|\s*)\}\}`
	defaultOSReferencePattern = `\{\{\s*@([^}]*)\}\}`
)

// grammar is the compiled reference syntax the tokenizer scans with: one pattern
// for internal {{VAR}} references and one for OS {{@VAR}} references. Both are
// anchored, so a match is only ever accepted at the current scan position, and the
// OS pattern is tried first so an overlapping custom pattern resolves OS-first.
type grammar struct {
	// reference matches an internal {{VAR}} reference; group 1 is the variable name.
	reference *regexp.Regexp
	// osReference matches an OS {{@VAR}} reference; group 1 is the variable name.
	osReference *regexp.Regexp
}

// defaultGrammar is the built-in reference syntax, used by callers that configure
// no custom patterns and by the package-level tokenizer helpers.
var defaultGrammar = mustGrammar(defaultReferencePattern, defaultOSReferencePattern)

// newGrammar compiles the internal and OS reference patterns, falling back to the
// built-in default for an empty pattern. It is the config-time validation seam:
// an invalid or capture-group-less pattern is a clear error here, before any
// value is substituted. Each pattern must expose at least one capture group,
// whose first group is taken as the variable name.
func newGrammar(referencePattern, osReferencePattern string) (*grammar, error) {
	reference, err := compileReferencePattern(
		referencePattern, defaultReferencePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("reference pattern: %w", err)
	}
	osReference, err := compileReferencePattern(
		osReferencePattern, defaultOSReferencePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("os reference pattern: %w", err)
	}
	return &grammar{reference: reference, osReference: osReference}, nil
}

// mustGrammar builds a grammar from patterns known to be valid, panicking
// otherwise. It backs defaultGrammar, whose patterns are compile-time constants.
func mustGrammar(referencePattern, osReferencePattern string) *grammar {
	g, err := newGrammar(referencePattern, osReferencePattern)
	if err != nil {
		panic(err)
	}
	return g
}

// compileReferencePattern compiles pattern (or fallback when pattern is empty),
// anchoring it to the scan position and requiring a capture group for the variable
// name. Anchoring with \A means the match must begin exactly where the tokenizer
// is looking, so the scanner advances one position at a time rather than letting a
// pattern match arbitrarily far ahead.
func compileReferencePattern(pattern, fallback string) (*regexp.Regexp, error) {
	if pattern == "" {
		pattern = fallback
	}
	// Validate the raw pattern first so a syntax error names the user's pattern
	// rather than the internal anchor wrapper.
	if _, err := regexp.Compile(pattern); err != nil {
		return nil, err
	}
	// Anchor the validated pattern to the scan position; the wrap cannot introduce
	// a new syntax error over an already-valid pattern.
	compiled := regexp.MustCompile(`\A(?:` + pattern + `)`)
	if compiled.NumSubexp() < 1 {
		return nil, fmt.Errorf(
			"pattern %q must contain a capture group for the variable name", pattern,
		)
	}
	return compiled, nil
}

// matchAt reports whether a reference begins at position i in value. It tries the
// OS pattern first so an OS reference is never mistaken for an internal one, and
// returns the trimmed variable name, the matched width, and the token kind. A
// zero-width match is rejected so the scanner always makes progress.
func (g *grammar) matchAt(
	value string, i int,
) (name string, width int, kind tokenKind, ok bool) {
	sub := value[i:]
	if m := g.osReference.FindStringSubmatchIndex(sub); len(m) > 0 && m[1] > 0 {
		return strings.TrimSpace(sub[m[2]:m[3]]), m[1], tokenOSRef, true
	}
	if m := g.reference.FindStringSubmatchIndex(sub); len(m) > 0 && m[1] > 0 {
		return strings.TrimSpace(sub[m[2]:m[3]]), m[1], tokenInternalRef, true
	}
	return "", 0, 0, false
}

// tokenize scans a value into literal spans and reference tokens, honoring the
// backslash escape that renders the following reference literally. Text that
// matches no pattern — including an unterminated opener — is carried through as
// literal text, so a value can never fail to tokenize.
func (g *grammar) tokenize(value string) []token {
	var tokens []token
	var lit strings.Builder

	flush := func() {
		if lit.Len() > 0 {
			tokens = append(tokens, token{kind: tokenLiteral, text: lit.String()})
			lit.Reset()
		}
	}

	for i := 0; i < len(value); {
		// A backslash immediately before a reference escapes it: emit the matched
		// text literally and drop the backslash.
		if value[i] == escape && i+1 < len(value) {
			if _, width, _, ok := g.matchAt(value, i+1); ok {
				lit.WriteString(value[i+1 : i+1+width])
				i += 1 + width
				continue
			}
		}

		if name, width, kind, ok := g.matchAt(value, i); ok {
			flush()
			tokens = append(tokens, token{kind: kind, text: name})
			i += width
			continue
		}

		lit.WriteByte(value[i])
		i++
	}

	flush()
	return tokens
}

// hasReferences reports whether value contains at least one reference, so a
// diagnoser can classify it as a variable substitution rather than a plain value.
func (g *grammar) hasReferences(value string) bool {
	for _, tok := range g.tokenize(value) {
		if tok.kind != tokenLiteral {
			return true
		}
	}
	return false
}

// hasEscape reports whether value carries an escaped reference, so the
// substitution stage strips the backslash even though the value holds no live
// reference. A diagnoser uses this to route an escape-only value through the
// engine so its revealed value matches what run and get produce.
func (g *grammar) hasEscape(value string) bool {
	for i := 0; i+1 < len(value); i++ {
		if value[i] != escape {
			continue
		}
		if _, _, _, ok := g.matchAt(value, i+1); ok {
			return true
		}
	}
	return false
}

// tokenize scans value with the built-in reference grammar. It preserves the
// package-level entry point used where no custom grammar is in play.
func tokenize(value string) []token {
	return defaultGrammar.tokenize(value)
}
