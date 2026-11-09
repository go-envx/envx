package diagnostic

import "github.com/go-envx/envx/app/internal/lib/severity"

// Code is a stable identifier for an application diagnostic condition.
type Code string

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
