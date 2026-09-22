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

// defaultReferencePattern is the built-in reference grammar. It matches {{VAR}}
// for a variable reference; the first capture group is the variable name, which
// the tokenizer trims. It stops at the first "}}" so a value can hold literal text
// after the reference. A workspace overrides the pattern via the reference-pattern
// setting; the exact sigil is therefore not load-bearing.
const defaultReferencePattern = `\{\{([^}]*)\}\}`

// grammar is the compiled reference syntax the tokenizer scans with: one pattern
// for {{VAR}} references. It is anchored, so a match is only ever accepted at the
// current scan position.
type grammar struct {
	// reference matches a {{VAR}} reference; group 1 is the variable name.
	reference *regexp.Regexp
}

// defaultGrammar is the built-in reference syntax, used by callers that configure
// no custom pattern and by the package-level tokenizer helpers.
var defaultGrammar = mustGrammar(defaultReferencePattern)

// newGrammar compiles the reference pattern, falling back to the built-in default
// for an empty pattern. It is the config-time validation seam: an invalid or
// capture-group-less pattern is a clear error here, before any value is
// substituted. The pattern must expose at least one capture group, whose first
// group is taken as the variable name.
func newGrammar(referencePattern string) (*grammar, error) {
	reference, err := compileReferencePattern(
		referencePattern, defaultReferencePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("reference pattern: %w", err)
	}
	return &grammar{reference: reference}, nil
}

// mustGrammar builds a grammar from a pattern known to be valid, panicking
// otherwise. It backs defaultGrammar, whose pattern is a compile-time constant.
func mustGrammar(referencePattern string) *grammar {
	g, err := newGrammar(referencePattern)
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

// matchAt reports whether a reference begins at position i in value, returning the
// trimmed variable name and the matched width. A zero-width match is rejected so
// the scanner always makes progress.
func (g *grammar) matchAt(value string, i int) (name string, width int, ok bool) {
	sub := value[i:]
	if m := g.reference.FindStringSubmatchIndex(sub); len(m) > 0 && m[1] > 0 {
		return strings.TrimSpace(sub[m[2]:m[3]]), m[1], true
	}
	return "", 0, false
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
			if _, width, ok := g.matchAt(value, i+1); ok {
				lit.WriteString(value[i+1 : i+1+width])
				i += 1 + width
				continue
			}
		}

		if name, width, ok := g.matchAt(value, i); ok {
			flush()
			tokens = append(tokens, token{kind: tokenReference, text: name})
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
		if _, _, ok := g.matchAt(value, i+1); ok {
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
