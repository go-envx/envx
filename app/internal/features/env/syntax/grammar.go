package syntax

import (
	"fmt"
	"regexp"
	"strings"
)

// escape makes the following reference token a literal: a backslash immediately
// before a reference match renders the matched text verbatim and drops the
// backslash, so a value can carry an untouched template or sigil.
const escape = '\\'

// defaultReferencePattern is the built-in reference grammar. It matches {{VAR}}
// for a variable reference; the first capture group is the variable name, which
// the tokenizer trims. It stops at the first "}}" so a value can hold literal text
// after the reference. A workspace overrides the pattern via settings; the exact
// sigil is therefore not load-bearing.
const defaultReferencePattern = `\{\{([^}]*)\}\}`

// Grammar is the compiled reference syntax the tokenizer scans with: one pattern
// for variable references. It is anchored, so a match is only ever accepted at
// the current scan position.
type Grammar struct {
	// reference matches a variable reference; group 1 is the variable name.
	reference *regexp.Regexp
}

// DefaultGrammar is the built-in reference syntax, used by callers that configure
// no custom pattern and by package-level tokenizer helpers.
var DefaultGrammar = mustNewGrammar(GrammarParams{
	ReferencePattern: defaultReferencePattern,
})

// GrammarParams specifies the input parameters for compiling a Grammar.
type GrammarParams struct {
	// ReferencePattern is the regular expression matching variable references.
	// When empty, the built-in default pattern matching {{VAR}} is used.
	ReferencePattern string
}

// NewGrammar compiles the reference pattern from params, falling back to the built-in
// default for an empty pattern. It is the config-time validation seam: an invalid or
// capture-group-less pattern returns a clear error here, before any value is
// substituted. The pattern must expose at least one capture group, whose first
// group is taken as the variable name.
func NewGrammar(params GrammarParams) (*Grammar, error) {
	reference, err := compileReferencePattern(
		params.ReferencePattern, defaultReferencePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("reference pattern: %w", err)
	}
	return &Grammar{reference: reference}, nil
}

// mustNewGrammar builds a grammar from params known to be valid, panicking
// otherwise. It backs DefaultGrammar, whose pattern is a compile-time constant.
func mustNewGrammar(params GrammarParams) *Grammar {
	g, err := NewGrammar(params)
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
func (g *Grammar) matchAt(value string, i int) (name string, width int, ok bool) {
	sub := value[i:]
	if m := g.reference.FindStringSubmatchIndex(sub); len(m) > 0 && m[1] > 0 {
		return strings.TrimSpace(sub[m[2]:m[3]]), m[1], true
	}
	return "", 0, false
}

// Tokenize scans a value into literal spans and reference tokens, honoring the
// backslash escape that renders the following reference literally. Text that
// matches no pattern — including an unterminated opener — is carried through as
// literal text, so a value can never fail to tokenize.
func (g *Grammar) Tokenize(value string) []Token {
	var tokens []Token
	var lit strings.Builder

	flush := func() {
		if lit.Len() > 0 {
			tokens = append(tokens, Token{Kind: TokenLiteral, Text: lit.String()})
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
			tokens = append(tokens, Token{Kind: TokenReference, Text: name})
			i += width
			continue
		}

		lit.WriteByte(value[i])
		i++
	}

	flush()
	return tokens
}

// HasReferences reports whether value contains at least one reference.
func (g *Grammar) HasReferences(value string) bool {
	for _, tok := range g.Tokenize(value) {
		if tok.Kind != TokenLiteral {
			return true
		}
	}
	return false
}

// HasEscape reports whether value carries an escaped reference, so a substitution
// stage can strip the backslash even though the value holds no live reference.
func (g *Grammar) HasEscape(value string) bool {
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

// Tokenize scans value with the default reference grammar. It preserves the
// package-level entry point used where no custom grammar is in play.
func Tokenize(value string) []Token {
	return DefaultGrammar.Tokenize(value)
}
