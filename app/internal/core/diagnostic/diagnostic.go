package diagnostic

import "github.com/go-envx/envx/app/internal/lib/severity"

// Code is a stable identifier for an application diagnostic condition.
type Code string

// OK marks a value that resolved with no problem; it is never a reportable finding.
const OK Code = "OK"

// Definition describes a reportable code and its built-in severity.
type Definition struct {
	Code Code
	// Level is the built-in severity; configuration may override it via
	// Resolve. severity.None means the check is disabled.
	Level   severity.Level
	Summary string
}

// Definitions returns a copy of every reportable definition in stable order.
func Definitions() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	return out
}

// Lookup returns the definition for code, and false for OK or an unknown code.
func Lookup(code Code) (Definition, bool) {
	for _, def := range definitions {
		if def.Code == code {
			return def, true
		}
	}
	return Definition{}, false
}

// Finding represents a structured diagnostic observation without secret material.
// The zero finding represents no finding.
type Finding struct {
	Code        Code
	Level       severity.Level
	Project     string
	Environment string
	Key         string
	Message     string
}

// IsZero reports whether the finding is the zero value (no finding).
func (f Finding) IsZero() bool {
	return f == Finding{}
}
