// Package status defines the canonical validation status codes shared by the
// validate engine and the secrets/envmerge diagnosers, together with each code's
// default reporting severity.
//
// The codes were previously duplicated across three packages; centralizing them
// here keeps one definition per condition and gives validate a single table to
// resolve a code's severity against. The package is a dependency-free leaf so
// both secrets and validate can import it without a cycle. No code or severity
// ever carries secret or private-key material.
package status

import "strings"

// Canonical status codes. Each identifies one condition validate can report;
// OK marks a value that resolved with no problem and never becomes a finding.
const (
	// OK marks a value that resolved successfully.
	OK = "OK"

	// SecretIsNotEncrypted marks a stored value that is not a ciphertext envelope.
	SecretIsNotEncrypted = "SECRET_IS_NOT_ENCRYPTED"
	// SecretAlgorithmMismatch marks a stored value encrypted under an algorithm the
	// configured cipher cannot use.
	SecretAlgorithmMismatch = "SECRET_ALGORITHM_MISMATCH"
	// SecretIsNotReferenced marks a stored value no environment references.
	SecretIsNotReferenced = "SECRET_IS_NOT_REFERENCED"
	// PublicKeyIsMissing marks a group that has private-key material but no stored
	// public key.
	PublicKeyIsMissing = "PUBLIC_KEY_IS_MISSING"
	// PrivateKeyIsInvalid marks a group whose private key is malformed or does not
	// match its stored public key.
	PrivateKeyIsInvalid = "PRIVATE_KEY_IS_INVALID"
	// PrivateKeyIsUnavailable marks a group with no private key available in this
	// context.
	PrivateKeyIsUnavailable = "PRIVATE_KEY_IS_UNAVAILABLE"

	// SecretReferenceNotFound marks a reference to a value absent from the store.
	SecretReferenceNotFound = "SECRET_REFERENCE_NOT_FOUND"
	// InvalidSecretReference marks a value whose secret-reference grammar is
	// malformed.
	InvalidSecretReference = "INVALID_SECRET_REFERENCE"
	// SecretReferenceIsUnresolved marks a reference that passed every store check
	// yet still failed to decrypt, such as a corrupt ciphertext.
	SecretReferenceIsUnresolved = "SECRET_REFERENCE_IS_UNRESOLVED"
	// CircularVariableReference marks a substitution with a reference cycle.
	CircularVariableReference = "CIRCULAR_VARIABLE_REFERENCE"
	// UnresolvedVariableReference marks a substitution with a missing internal or OS
	// reference.
	UnresolvedVariableReference = "UNRESOLVED_VARIABLE_REFERENCE"
	// PropertyNotDeclaredInBase marks an overlay key its namespace base file never
	// declares.
	PropertyNotDeclaredInBase = "PROPERTY_NOT_DECLARED_IN_BASE"
)

// Severity is the reporting level configured or defaulted for a status code. Off
// suppresses the check, Warn reports it without failing by default, and Error
// fails the run.
type Severity string

const (
	// Off suppresses a check entirely; it produces no finding.
	Off Severity = "off"
	// Warn reports a finding that does not fail the run unless --strict is set.
	Warn Severity = "warn"
	// Error reports a finding that always fails the run.
	Error Severity = "error"
)

// defaults holds the built-in severity for every reportable code. Everything is
// an error except two checks: a private key that is merely unavailable in this
// context (normal on a developer laptop, so a warning), and the base-declaration
// check, an opt-in best practice that defaults to off so it never surprises an
// existing workspace — a team enables it by setting property_not_declared_in_base
// in the validate block.
var defaults = map[string]Severity{
	SecretIsNotEncrypted:        Error,
	SecretAlgorithmMismatch:     Error,
	SecretIsNotReferenced:       Error,
	PublicKeyIsMissing:          Error,
	PrivateKeyIsInvalid:         Error,
	PrivateKeyIsUnavailable:     Warn,
	SecretReferenceNotFound:     Error,
	InvalidSecretReference:      Error,
	SecretReferenceIsUnresolved: Error,
	CircularVariableReference:   Error,
	UnresolvedVariableReference: Error,
	PropertyNotDeclaredInBase:   Off,
}

// DefaultSeverity returns the built-in severity for code, and false for a code
// that is not a reportable finding (such as OK or an unknown code). Callers grade
// only reportable codes, so a false result means "no finding".
func DefaultSeverity(code string) (Severity, bool) {
	severity, ok := defaults[code]
	return severity, ok
}

// ParseSeverity converts a configured value ("off", "warn", or "error") into a
// Severity, rejecting anything else so a typo in envx.yaml fails loudly.
func ParseSeverity(value string) (Severity, error) {
	switch Severity(value) {
	case Off, Warn, Error:
		return Severity(value), nil
	default:
		return "", &InvalidSeverityError{Value: value}
	}
}

// InvalidSeverityError reports a validate severity that is not off, warn, or
// error. It names the offending value without any secret material.
type InvalidSeverityError struct {
	// Value is the rejected severity string.
	Value string
}

// Error describes the rejected severity and the accepted set.
func (e *InvalidSeverityError) Error() string {
	return "invalid severity " + e.Value + " (want off, warn, or error)"
}

// UnknownCheckError reports a validate config key that is not a known check code.
// It names the offending key so a typo in envx.yaml is easy to find.
type UnknownCheckError struct {
	// Key is the unrecognized, lowercased check name from the config.
	Key string
}

// Error describes the unrecognized check key.
func (e *UnknownCheckError) Error() string {
	return "unknown validate check " + e.Key
}

// Resolve converts a raw validate severity config — keyed by lowercased check
// code (e.g. "secret_is_not_referenced") with an off/warn/error value — into a
// map keyed by canonical status code. It rejects an unknown check key or an
// invalid severity so a typo in envx.yaml fails loudly. A nil or empty input
// yields a nil map, meaning "every code keeps its default".
func Resolve(raw map[string]string) (map[string]Severity, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]Severity, len(raw))
	for key, value := range raw {
		code := strings.ToUpper(key)
		if _, ok := defaults[code]; !ok {
			return nil, &UnknownCheckError{Key: key}
		}
		severity, err := ParseSeverity(value)
		if err != nil {
			return nil, err
		}
		out[code] = severity
	}
	return out, nil
}
