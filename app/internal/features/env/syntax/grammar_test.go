package syntax_test

import (
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
)

// TestNewDefaults verifies an empty pattern falls back to the built-in
// grammar, tokenizing {{VAR}} exactly as the default constant does.
func TestNewDefaults(t *testing.T) {
	t.Parallel()

	g, err := syntax.NewGrammar(syntax.GrammarParams{})
	if err != nil {
		t.Fatalf("NewGrammar(default): %v", err)
	}

	got := g.Tokenize("{{SCHEME}}://{{HOST}}")
	want := []syntax.Token{
		{Kind: syntax.TokenReference, Text: "SCHEME"},
		{Kind: syntax.TokenLiteral, Text: "://"},
		{Kind: syntax.TokenReference, Text: "HOST"},
	}
	if len(got) != len(want) {
		t.Fatalf("Tokenize = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("token[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestNewCustomPattern verifies a custom pattern defines the reference syntax
// and the engine composes values through the custom grammar.
func TestNewCustomPattern(t *testing.T) {
	t.Parallel()

	g, err := syntax.NewGrammar(syntax.GrammarParams{
		ReferencePattern: `\$\{([^}]*)\}`,
	})
	if err != nil {
		t.Fatalf("NewGrammar(custom): %v", err)
	}

	got := g.Tokenize("${SCHEME}://${HOST}")
	want := []syntax.Token{
		{Kind: syntax.TokenReference, Text: "SCHEME"},
		{Kind: syntax.TokenLiteral, Text: "://"},
		{Kind: syntax.TokenReference, Text: "HOST"},
	}
	if len(got) != len(want) {
		t.Fatalf("Tokenize = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("token[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// The default {{VAR}} syntax is inert under the custom grammar.
	if refs := g.HasReferences("{{SCHEME}}"); refs {
		t.Error("default {{VAR}} should not tokenize under a custom grammar")
	}
}

// TestNewInvalidPattern verifies an uncompilable pattern is an error naming the
// reference pattern, failing early at validation time.
func TestNewInvalidPattern(t *testing.T) {
	t.Parallel()

	if _, err := syntax.NewGrammar(syntax.GrammarParams{
		ReferencePattern: `(unterminated`,
	}); err == nil ||
		!strings.Contains(err.Error(), "reference pattern") {
		t.Errorf("pattern error = %v, want a reference pattern error", err)
	}
}

// TestNewMissingCaptureGroup verifies a pattern without a capture group is
// rejected, since the tokenizer extracts the variable name from the first group.
func TestNewMissingCaptureGroup(t *testing.T) {
	t.Parallel()

	if _, err := syntax.NewGrammar(syntax.GrammarParams{
		ReferencePattern: `\{\{[^}]*\}\}`,
	}); err == nil ||
		!strings.Contains(err.Error(), "capture group") {
		t.Errorf("error = %v, want a missing-capture-group error", err)
	}
}

// TestDefaultGrammar verifies the package-level DefaultGrammar is initialized.
func TestDefaultGrammar(t *testing.T) {
	t.Parallel()

	if syntax.DefaultGrammar == nil {
		t.Fatal("DefaultGrammar is nil")
	}
}

// TestTokenize verifies the scanner splits literals from references, trims
// whitespace inside the braces, honors the \{{ escape, and treats an unterminated
// opener as literal text.
func TestTokenize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  []syntax.Token
	}{
		{
			name:  "plain literal",
			value: "postgresql://db:5432",
			want:  []syntax.Token{{Kind: syntax.TokenLiteral, Text: "postgresql://db:5432"}},
		},
		{
			name:  "reference",
			value: "{{HOST}}",
			want:  []syntax.Token{{Kind: syntax.TokenReference, Text: "HOST"}},
		},
		{
			name:  "trims whitespace",
			value: "{{  HOST  }}",
			want:  []syntax.Token{{Kind: syntax.TokenReference, Text: "HOST"}},
		},
		{
			name:  "multiple references and literals",
			value: "{{SCHEME}}://{{HOST}}:{{PORT}}",
			want: []syntax.Token{
				{Kind: syntax.TokenReference, Text: "SCHEME"},
				{Kind: syntax.TokenLiteral, Text: "://"},
				{Kind: syntax.TokenReference, Text: "HOST"},
				{Kind: syntax.TokenLiteral, Text: ":"},
				{Kind: syntax.TokenReference, Text: "PORT"},
			},
		},
		{
			name:  "escaped opener is literal",
			value: `\{{HOST}}`,
			want:  []syntax.Token{{Kind: syntax.TokenLiteral, Text: "{{HOST}}"}},
		},
		{
			name:  "escape then live reference",
			value: `\{{LITERAL}} {{HOST}}`,
			want: []syntax.Token{
				{Kind: syntax.TokenLiteral, Text: "{{LITERAL}} "},
				{Kind: syntax.TokenReference, Text: "HOST"},
			},
		},
		{
			name:  "unterminated opener is literal",
			value: "a{{b",
			want:  []syntax.Token{{Kind: syntax.TokenLiteral, Text: "a{{b"}},
		},
		{
			name:  "lone backslash is literal",
			value: `a\b`,
			want:  []syntax.Token{{Kind: syntax.TokenLiteral, Text: `a\b`}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := syntax.Tokenize(tt.value)
			if len(got) != len(tt.want) {
				t.Fatalf("Tokenize(%q) = %v, want %v", tt.value, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestHasReferences verifies reference detection on literals and templates.
func TestHasReferences(t *testing.T) {
	t.Parallel()

	g := syntax.DefaultGrammar
	if g.HasReferences("plain text") {
		t.Error("plain text should not have references")
	}
	if !g.HasReferences("prefix {{VAR}} suffix") {
		t.Error("string with {{VAR}} should have references")
	}
	if g.HasReferences(`escaped \{{VAR}} only`) {
		t.Error("escaped reference should not be detected as an active reference")
	}
}

// TestHasEscape verifies escape detection on raw values.
func TestHasEscape(t *testing.T) {
	t.Parallel()

	g := syntax.DefaultGrammar
	if g.HasEscape("plain text") {
		t.Error("plain text should not have escape")
	}
	if g.HasEscape("prefix {{VAR}} suffix") {
		t.Error("unescaped reference should not have escape")
	}
	if !g.HasEscape(`prefix \{{VAR}} suffix`) {
		t.Error("escaped reference should have escape")
	}
	if g.HasEscape(`lone backslash \ without reference`) {
		t.Error("lone backslash should not report escape")
	}
}

// TestTokenKindString verifies the String method on TokenKind.
func TestTokenKindString(t *testing.T) {
	t.Parallel()

	if got := syntax.TokenLiteral.String(); got != "literal" {
		t.Errorf("TokenLiteral.String() = %q, want %q", got, "literal")
	}
	if got := syntax.TokenReference.String(); got != "reference" {
		t.Errorf("TokenReference.String() = %q, want %q", got, "reference")
	}
	if got := syntax.TokenKind(99).String(); got != "TokenKind(99)" {
		t.Errorf("TokenKind(99).String() = %q, want %q", got, "TokenKind(99)")
	}
}
