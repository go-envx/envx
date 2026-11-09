package syntax

import (
	"fmt"
	"regexp"
	"strings"
)

// escape makes the following reference literal: the backslash is dropped and the
// matched text is kept verbatim.
const escape = '\\'

// defaultPattern matches {{VAR}}; group 1 is the variable name.
const defaultPattern = `\{\{([^}]*)\}\}`

// defaultReference is the compiled built-in reference pattern.
var defaultReference = regexp.MustCompile(anchor(defaultPattern))

// Grammar is a compiled reference syntax. The zero Grammar uses the built-in
// {{VAR}} syntax.
type Grammar struct {
	// reference matches a reference at the scan position; group 1 is the name.
	reference *regexp.Regexp
}

// token is one scanned span: literal text or a reference name.
type token struct {
	// text is the literal content or the trimmed reference name.
	text string
	// reference marks a reference span.
	reference bool
}

// Compile compiles a reference pattern whose first capture group is the
// variable name. An empty pattern selects the built-in syntax.
func Compile(pattern string) (*Grammar, error) {
	if pattern == "" {
		return &Grammar{reference: defaultReference}, nil
	}
	// Validate the raw pattern so a syntax error names the user's pattern rather
	// than the anchored wrapper.
	if _, err := regexp.Compile(pattern); err != nil {
		return nil, fmt.Errorf("reference pattern: %w", err)
	}
	compiled := regexp.MustCompile(anchor(pattern))
	if compiled.NumSubexp() < 1 {
		return nil, fmt.Errorf(
			"reference pattern: %q must contain a capture group for the variable name",
			pattern,
		)
	}
	return &Grammar{reference: compiled}, nil
}

// References returns the variable names referenced by text in order, excluding
// escaped references.
func (g Grammar) References(text string) []string {
	var names []string
	for _, tok := range g.tokenize(text) {
		if tok.reference {
			names = append(names, tok.text)
		}
	}
	return names
}

// HasEscape reports whether text carries an escaped reference, so substitution
// would change it even without a live reference.
func (g Grammar) HasEscape(text string) bool {
	for i := 0; i+1 < len(text); i++ {
		if text[i] != escape {
			continue
		}
		if _, _, ok := g.matchAt(text, i+1); ok {
			return true
		}
	}
	return false
}

// Substitute replaces every reference in text with lookup's result and renders
// escaped references literally. The first lookup error is returned.
func (g Grammar) Substitute(
	text string, lookup func(name string) (string, error),
) (string, error) {
	var b strings.Builder
	for _, tok := range g.tokenize(text) {
		if !tok.reference {
			b.WriteString(tok.text)
			continue
		}
		resolved, err := lookup(tok.text)
		if err != nil {
			return "", err
		}
		b.WriteString(resolved)
	}
	return b.String(), nil
}

// tokenize scans text into literal and reference spans. Text matching no
// reference, including an unterminated opener, is literal.
func (g Grammar) tokenize(text string) []token {
	var tokens []token
	var lit strings.Builder

	flush := func() {
		if lit.Len() > 0 {
			tokens = append(tokens, token{text: lit.String()})
			lit.Reset()
		}
	}

	for i := 0; i < len(text); {
		if text[i] == escape && i+1 < len(text) {
			if _, width, ok := g.matchAt(text, i+1); ok {
				lit.WriteString(text[i+1 : i+1+width])
				i += 1 + width
				continue
			}
		}
		if name, width, ok := g.matchAt(text, i); ok {
			flush()
			tokens = append(tokens, token{text: name, reference: true})
			i += width
			continue
		}
		lit.WriteByte(text[i])
		i++
	}

	flush()
	return tokens
}

// matchAt reports whether a reference begins at position i, returning its
// trimmed name and width. Zero-width matches are rejected so scanning advances.
func (g Grammar) matchAt(text string, i int) (name string, width int, ok bool) {
	reference := g.reference
	if reference == nil {
		reference = defaultReference
	}
	sub := text[i:]
	m := reference.FindStringSubmatchIndex(sub)
	if len(m) == 0 || m[1] == 0 {
		return "", 0, false
	}
	// An optional name group may not participate in the match.
	if m[2] < 0 {
		return "", m[1], true
	}
	return strings.TrimSpace(sub[m[2]:m[3]]), m[1], true
}

// anchor wraps pattern so it only matches at the scan position.
func anchor(pattern string) string {
	return `\A(?:` + pattern + `)`
}
