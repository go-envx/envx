package diagnostic

// Reportable diagnostic codes. The strings are a public contract and must not change.
// Keep these codes consistent with the definitions in registry_def.go.
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

	// InvalidExternalReference marks a malformed reference for an enabled
	// external provider.
	InvalidExternalReference Code = "INVALID_EXTERNAL_REFERENCE"

	// ExternalProviderNotEnabled marks a value using the scheme of an undeclared
	// external provider; the value passes through unchanged.
	ExternalProviderNotEnabled Code = "EXTERNAL_PROVIDER_NOT_ENABLED"

	// ExternalProviderUnavailable marks a declared provider whose CLI or
	// credentials are missing, or that timed out earlier in the operation.
	ExternalProviderUnavailable Code = "EXTERNAL_PROVIDER_UNAVAILABLE"

	// ExternalReferenceUnresolved marks a provider call that failed, returned no
	// value, or matched more than one secret.
	ExternalReferenceUnresolved Code = "EXTERNAL_REFERENCE_UNRESOLVED"
)
