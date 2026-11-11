package diagnostic

import "github.com/go-envx/envx/app/internal/lib/severity"

// Definitions of diagnostic codes and their severity levels.
// Keep these definitions consistent with the codes in registry.go.
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
	{
		Code:    InvalidExternalReference,
		Level:   severity.Error,
		Summary: "a malformed reference for an enabled external provider",
	},
	{
		Code:    ExternalProviderNotEnabled,
		Level:   severity.Warn,
		Summary: "a value using the scheme of an undeclared external provider",
	},
	{
		Code:    ExternalProviderUnavailable,
		Level:   severity.Error,
		Summary: "a declared external provider that cannot be used",
	},
	{
		Code:    ExternalReferenceUnresolved,
		Level:   severity.Error,
		Summary: "an external reference the provider could not resolve",
	},
}
