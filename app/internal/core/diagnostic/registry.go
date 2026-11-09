package diagnostic

import "github.com/go-envx/envx/app/internal/lib/severity"

// Reportable diagnostic codes. The strings are a public contract and must not change.
//
//nolint:gosec // G101: code names, not credentials.
const (
	// SecretIsNotEncrypted marks a stored value that is not a ciphertext envelope.
	SecretIsNotEncrypted Code = "SECRET_IS_NOT_ENCRYPTED"

	// SecretAlgorithmMismatch marks a stored value encrypted under an algorithm
	// the configured cipher cannot use.
	SecretAlgorithmMismatch Code = "SECRET_ALGORITHM_MISMATCH"

	// SecretIsNotReferenced marks a stored value no environment references.
	SecretIsNotReferenced Code = "SECRET_IS_NOT_REFERENCED"

	// PublicKeyIsMissing marks a group that has private-key material but no
	// stored public key.
	PublicKeyIsMissing Code = "PUBLIC_KEY_IS_MISSING"

	// PrivateKeyIsInvalid marks a group whose private key is malformed or does
	// not match its stored public key.
	PrivateKeyIsInvalid Code = "PRIVATE_KEY_IS_INVALID"

	// PrivateKeyIsUnavailable marks a group with no private key available in
	// this context. It warns by default because this is normal on a developer
	// machine.
	PrivateKeyIsUnavailable Code = "PRIVATE_KEY_IS_UNAVAILABLE"

	// SecretReferenceNotFound marks a reference to a value absent from the store.
	SecretReferenceNotFound Code = "SECRET_REFERENCE_NOT_FOUND"

	// InvalidSecretReference marks a value whose secret-reference grammar is
	// malformed.
	InvalidSecretReference Code = "INVALID_SECRET_REFERENCE"

	// SecretReferenceIsUnresolved marks a reference that passed every store
	// check yet still failed to decrypt, such as a corrupt ciphertext.
	SecretReferenceIsUnresolved Code = "SECRET_REFERENCE_IS_UNRESOLVED"

	// CircularVariableReference marks a substitution with a reference cycle.
	CircularVariableReference Code = "CIRCULAR_VARIABLE_REFERENCE"

	// UnresolvedVariableReference marks a substitution with a missing internal
	// or OS reference.
	UnresolvedVariableReference Code = "UNRESOLVED_VARIABLE_REFERENCE"

	// PropertyNotDeclaredInBase marks an overlay key its namespace base file
	// never declares. It is an opt-in best practice, so it is off by default.
	PropertyNotDeclaredInBase Code = "PROPERTY_NOT_DECLARED_IN_BASE"
)

var definitions = []Definition{
	{
		Code:    SecretIsNotEncrypted,
		Level:   severity.Error,
		Summary: "a stored value left unencrypted",
	},
	{
		Code:    SecretAlgorithmMismatch,
		Level:   severity.Error,
		Summary: "a value stored under a non-configured algorithm",
	},
	{
		Code:    SecretIsNotReferenced,
		Level:   severity.Error,
		Summary: "a stored value no environment references",
	},
	{
		Code:    PublicKeyIsMissing,
		Level:   severity.Error,
		Summary: "a group with stored secrets but no public key",
	},
	{
		Code:    PrivateKeyIsInvalid,
		Level:   severity.Error,
		Summary: "a group whose private key is malformed or mismatched",
	},
	{
		Code:    PrivateKeyIsUnavailable,
		Level:   severity.Warn,
		Summary: "a group with no private key available in this context",
	},
	{
		Code:    SecretReferenceNotFound,
		Level:   severity.Error,
		Summary: "a reference to a value absent from the store",
	},
	{
		Code:    InvalidSecretReference,
		Level:   severity.Error,
		Summary: "a malformed secret reference",
	},
	{
		Code:    SecretReferenceIsUnresolved,
		Level:   severity.Error,
		Summary: "a reference that passed every store check yet failed to decrypt",
	},
	{
		Code:    CircularVariableReference,
		Level:   severity.Error,
		Summary: "a variable substitution cycle",
	},
	{
		Code:    UnresolvedVariableReference,
		Level:   severity.Error,
		Summary: "a substitution of an undefined variable",
	},
	{
		Code:    PropertyNotDeclaredInBase,
		Level:   severity.None,
		Summary: "an overlay key its namespace base file never declares",
	},
}
