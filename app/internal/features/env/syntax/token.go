package syntax

import "fmt"

// TokenKind classifies a span produced by the tokenizer.
type TokenKind int

const (
	// TokenLiteral is verbatim text carried through untouched.
	TokenLiteral TokenKind = iota
	// TokenReference is a reference to a variable in the composed environment.
	TokenReference
)

// String returns a human-readable representation of the token kind.
func (k TokenKind) String() string {
	switch k {
	case TokenLiteral:
		return "literal"
	case TokenReference:
		return "reference"
	default:
		return fmt.Sprintf("TokenKind(%d)", int(k))
	}
}

// Token is one span of a scanned value: literal text, or a reference whose text
// is the trimmed variable name.
type Token struct {
	// Kind classifies the span as literal text or a variable reference.
	Kind TokenKind
	// Text is literal content for a literal span, or the variable name for a
	// reference span.
	Text string
}
