package envmerge

// Kind classifies how a value is materialized.
type Kind string

const (
	// KindConfigValue is a plain configuration value with no dereferencing.
	KindConfigValue Kind = "config"
	// KindSecretReference is a reference resolved from the secrets store.
	KindSecretReference Kind = "secret"
	// KindCommandSubstitution is reserved for command substitution; it is
	// defined but not yet produced.
	KindCommandSubstitution Kind = "command_substitution"
)

// Severity ranks a resolution outcome.
type Severity string

const (
	// SeverityOK marks a value that resolved successfully.
	SeverityOK Severity = "ok"
	// SeverityWarning marks a non-fatal outcome, such as an unavailable key.
	SeverityWarning Severity = "warning"
	// SeverityError marks a failed outcome, such as a dangling reference.
	SeverityError Severity = "error"
)

// Resolution is the non-fatal, dry-run outcome of materializing one value.
// Resolved carries plaintext only when reveal was requested and succeeded, and
// Code and Message stay free of private-key or secret material.
type Resolution struct {
	// Kind classifies how the value is materialized.
	Kind Kind
	// Severity ranks the outcome.
	Severity Severity
	// Code is a stable, machine-classifiable status identifier.
	Code string
	// Message is a human-readable status description free of secret material.
	Message string
	// Resolved carries plaintext only when reveal was requested and succeeded.
	Resolved string
	// HasResolved reports whether Resolved holds a materialized value.
	HasResolved bool
}

// ValueDiagnoser augments a ValueResolver with structured dry-run resolution. It
// classifies a value and reports status without aborting, discarding any
// materialized plaintext unless reveal was requested.
type ValueDiagnoser interface {
	// Diagnose classifies value in the active environment and reports its
	// non-fatal resolution outcome.
	Diagnose(value, environment string) Resolution
}
