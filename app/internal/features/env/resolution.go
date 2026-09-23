package env

import (
	"github.com/go-envx/envx/app/internal/shared/value"
	"github.com/go-envx/envx/app/internal/utils/severity"
)

// Kind classifies how a value is materialized.
type Kind = value.Kind

const (
	// KindConfigValue is a plain configuration value with no dereferencing.
	KindConfigValue = value.KindConfig
	// KindSecretReference is a reference resolved from the secrets store.
	KindSecretReference = value.KindSecret
	// KindVariableSubstitution marks a value composed from the resolved values of
	// other variables through {{VAR}} references.
	KindVariableSubstitution = value.KindVariable
)

// Severity ranks a resolution outcome.
type Severity = severity.Level

const (
	// SeverityOK marks a value that resolved successfully.
	SeverityOK = severity.OK
	// SeverityWarning marks a non-fatal outcome, such as an unavailable key.
	SeverityWarning = severity.Warn
	// SeverityError marks a failed outcome, such as a dangling reference.
	SeverityError = severity.Error
)

// Resolution is the non-fatal, dry-run outcome of materializing one value.
// It aliases value.Resolution to preserve backwards compatibility while decoupling
// features via internal/shared/value.
type Resolution = value.Resolution

// Value pairs a raw value with its resolution outcome.
type Value = value.Value

// ValueDiagnoser augments a ValueResolver with structured dry-run resolution.
// It aliases value.Evaluator so implementations can fulfill either contract.
type ValueDiagnoser interface {
	Diagnose(raw, environment string) Resolution
}
