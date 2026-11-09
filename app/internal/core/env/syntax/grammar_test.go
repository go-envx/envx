package syntax_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/env/syntax"
)

// TestCompile verifies pattern compilation, defaults, and rejection of invalid
// patterns.
func TestCompile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// pattern is the reference pattern to compile.
		pattern string
		// text is scanned for references when compilation succeeds.
		text string
		// want lists the expected reference names.
		want []string
		// wantErr is a substring of the expected compile error.
		wantErr string
	}{
		{
			name: "empty pattern uses default",
			text: "{{SCHEME}}://{{HOST}}",
			want: []string{"SCHEME", "HOST"},
		},
		{
			name:    "custom pattern",
			pattern: `\$\{([^}]*)\}`,
			text:    "${SCHEME}://${HOST} {{IGNORED}}",
			want:    []string{"SCHEME", "HOST"},
		},
		{
			name:    "invalid pattern",
			pattern: `(unterminated`,
			wantErr: "reference pattern",
		},
		{
			name:    "missing capture group",
			pattern: `\{\{[^}]*\}\}`,
			wantErr: "capture group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g, err := syntax.Compile(tt.pattern)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Compile(%q) error = %v, want %q", tt.pattern, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Compile(%q): %v", tt.pattern, err)
			}
			if got := g.References(tt.text); !slices.Equal(got, tt.want) {
				t.Errorf("References(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestReferences verifies scanning trims names, honors escapes, and treats
// unterminated openers as literal text.
func TestReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// text is the scanned value.
		text string
		// want lists the expected reference names.
		want []string
	}{
		{text: "postgresql://db:5432"},
		{text: "{{  HOST  }}", want: []string{"HOST"}},
		{text: "{{A}}{{B}}:{{A}}", want: []string{"A", "B", "A"}},
		{text: `\{{HOST}}`},
		{text: `\{{LITERAL}} {{HOST}}`, want: []string{"HOST"}},
		{text: "a{{b"},
		{text: `a\b`},
	}

	var g syntax.Grammar
	for _, tt := range tests {
		if got := g.References(tt.text); !slices.Equal(got, tt.want) {
			t.Errorf("References(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

// TestHasEscape verifies escape detection requires a following reference.
func TestHasEscape(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"plain text":                         false,
		"prefix {{VAR}} suffix":              false,
		`prefix \{{VAR}} suffix`:             true,
		`lone backslash \ without reference`: false,
	}

	var g syntax.Grammar
	for text, want := range tests {
		if got := g.HasEscape(text); got != want {
			t.Errorf("HasEscape(%q) = %v, want %v", text, got, want)
		}
	}
}

// TestSubstitute verifies references are replaced, escapes render literally,
// and lookup errors abort substitution.
func TestSubstitute(t *testing.T) {
	t.Parallel()

	values := map[string]string{"SCHEME": "https", "HOST": "db.local"}
	errMissing := errors.New("missing")
	lookup := func(name string) (string, error) {
		v, ok := values[name]
		if !ok {
			return "", errMissing
		}
		return v, nil
	}

	tests := []struct {
		// text is the substituted value.
		text string
		// want is the expected result.
		want string
		// wantErr is the expected lookup error.
		wantErr error
	}{
		{text: "{{SCHEME}}://{{ HOST }}:5432", want: "https://db.local:5432"},
		{text: `\{{HOST}} {{HOST}}`, want: "{{HOST}} db.local"},
		{text: "a{{b", want: "a{{b"},
		{text: "{{SCHEME}}{{UNKNOWN}}", wantErr: errMissing},
	}

	var g syntax.Grammar
	for _, tt := range tests {
		got, err := g.Substitute(tt.text, lookup)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("Substitute(%q) error = %v, want %v", tt.text, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("Substitute(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

// TestSubstituteOptionalGroup verifies a pattern whose name group does not
// participate yields an empty reference name instead of panicking.
func TestSubstituteOptionalGroup(t *testing.T) {
	t.Parallel()

	g, err := syntax.Compile(`<(\w+)?>`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	got, err := g.Substitute("a<>b", func(name string) (string, error) {
		return "[" + name + "]", nil
	})
	if err != nil || got != "a[]b" {
		t.Errorf("Substitute = %q, %v; want a[]b", got, err)
	}
}

// FuzzSubstitute verifies substitution never fails with a total lookup and leaves
// text without references or escapes unchanged.
func FuzzSubstitute(f *testing.F) {
	for _, seed := range []string{"", "{{A}}", `\{{A}}`, "a{{b", "{{ x }}y{{z}}"} {
		f.Add(seed)
	}
	var g syntax.Grammar
	f.Fuzz(func(t *testing.T, text string) {
		got, err := g.Substitute(text, func(string) (string, error) {
			return "value", nil
		})
		if err != nil {
			t.Fatalf("Substitute(%q): %v", text, err)
		}
		if len(g.References(text)) == 0 && !g.HasEscape(text) && got != text {
			t.Errorf("Substitute(%q) = %q, want unchanged", text, got)
		}
	})
}
