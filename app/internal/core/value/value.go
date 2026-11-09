package value

import "github.com/go-envx/envx/app/internal/core/diagnostic"

// Kind classifies the syntactic structure and source of an environment value.
type Kind uint8

const (
	// Config represents a static literal value with no dereferencing.
	Config Kind = iota
	// Secret represents a reference resolved from the secrets store.
	Secret
	// Variable represents a value composed from other variables via substitution.
	Variable
)

// String returns the canonical lowercase identifier for Kind.
func (k Kind) String() string {
	switch k {
	case Config:
		return "config"
	case Secret:
		return "secret"
	case Variable:
		return "variable"
	default:
		return "unknown"
	}
}

// Evaluation holds the outcome of evaluating an environment variable value.
// It carries the value kind, raw or resolved string representation, resolution status,
// and associated diagnostic finding. A masked secret can have a canonical reference
// in Value while Resolved remains false.
type Evaluation struct {
	Kind       Kind
	Value      string
	Resolved   bool
	Diagnostic diagnostic.Finding
}

// IsZero reports whether the evaluation is the zero value.
func (e Evaluation) IsZero() bool {
	return e == Evaluation{}
}
