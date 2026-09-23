// Package value defines the universal domain models, classification kinds, and
// evaluation contracts for environment variable values.
//
// An environment variable in a namespace file begins as a raw string (either a
// static config literal, a secret:// reference, or a {{VAR}} expression) and
// evaluates to a resolved outcome. This package is a dependency-free Shared
// Kernel leaf so domain slices (env, secrets, validate) can evaluate and pass
// around values without cyclic coupling.
package value

import (
	"encoding/json"
	"errors"

	"github.com/go-envx/envx/app/internal/utils/severity"
)

// Kind classifies the syntactic structure of an environment value.
type Kind int

const (
	// KindConfig is a static literal value with no dereferencing.
	KindConfig Kind = iota
	// KindSecret is a reference resolved from the secrets store (e.g. secret://group/key).
	KindSecret
	// KindVariable is a value composed from other variables via {{VAR}} substitution.
	KindVariable
)

// ErrInvalidKind reports an unrecognized value kind string.
var ErrInvalidKind = errors.New("invalid value kind")

// String returns the canonical lowercase identifier for Kind.
func (k Kind) String() string {
	switch k {
	case KindConfig:
		return "config"
	case KindSecret:
		return "secret"
	case KindVariable:
		return "variable"
	default:
		return "unknown"
	}
}

// MarshalJSON serializes Kind as its lowercase string representation.
func (k Kind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// UnmarshalJSON deserializes Kind from its lowercase string representation.
func (k *Kind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "config":
		*k = KindConfig
	case "secret":
		*k = KindSecret
	case "variable":
		*k = KindVariable
	default:
		return ErrInvalidKind
	}
	return nil
}

// Evaluation represents the outcome of evaluating or dry-running a value.
// It carries the classification status, severity, human-readable diagnostics,
// and optional materialized value.
type Evaluation struct {
	// Kind classifies how the value is defined (config, secret, or variable).
	Kind Kind
	// Severity ranks the outcome: None, OK, Warn, or Error.
	Severity severity.Level
	// Status is a stable, machine-classifiable status identifier (e.g. "OK",
	// "SECRET_REFERENCE_NOT_FOUND").
	Status string
	// Code aliases Status during refactoring.
	// TODO: remove after caller migration.
	Code string
	// StatusMessage is a human-readable explanation of status, free of secret material.
	StatusMessage string
	// Message aliases StatusMessage during refactoring.
	// TODO: remove after caller migration.
	Message string
	// Value holds the materialized plaintext when resolution succeeds and
	// reveal is enabled.
	Value string
	// Resolved aliases Value during refactoring.
	// TODO: remove after caller migration.
	Resolved string
	// IsResolved reports whether Value holds a materialized string.
	IsResolved bool
	// HasResolved aliases IsResolved during refactoring.
	// TODO: remove after caller migration.
	HasResolved bool
}

// Resolution aliases Evaluation for backwards compatibility during migration.
// TODO: remove after caller migration.
type Resolution = Evaluation

// Value pairs a raw environment entry with its syntactic classification and
// evaluation outcome.
type Value struct {
	// Kind classifies how the value is defined (config, secret, or variable).
	Kind Kind
	// Raw is the un-substituted literal string from the source file.
	Raw string
	// Evaluation holds the outcome of evaluating the value.
	Evaluation Evaluation
	// Resolution aliases Evaluation for backwards compatibility during migration.
	// TODO: remove after caller migration.
	Resolution Evaluation
}

// Resolver resolves a raw value directly into its materialized string at runtime,
// or returns an error if resolution fails.
type Resolver interface {
	// Resolve returns raw with any recognized reference or expression resolved
	// into plaintext, or an error if resolution fails.
	Resolve(raw, environment string) (string, error)
}

// Evaluator evaluates raw values in a dry-run fashion without aborting,
// classifying their syntactic kind and reporting a structured Evaluation.
type Evaluator interface {
	// Evaluate classifies raw in the given environment and reports its non-fatal
	// evaluation outcome.
	Evaluate(raw, environment string) Evaluation
}
