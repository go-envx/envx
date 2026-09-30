package secrets

// SetSecretResult reports the identity and storage location of an added or
// updated secret.
type SetSecretResult struct {
	// Location reports the origin or target descriptor of the secrets repository
	// where the secret was stored.
	Location string
	// Secret identifies the stored secret entry.
	Secret SecretReference
}

// GetSecretResult reports the decrypted value and origin location of a
// retrieved secret.
type GetSecretResult struct {
	// Location reports the origin or target descriptor of the secrets repository
	// where the secret was stored.
	Location string
	// Value is the decrypted secret plaintext.
	Value string
}

// DeleteSecretResult reports the identity and storage location of a removed secret.
type DeleteSecretResult struct {
	// Location reports the origin or target descriptor of the secrets repository
	// where the secret was deleted.
	Location string
	// Secret identifies the removed secret entry.
	Secret SecretReference
}

// EncryptSecretsResult reports changed secret identities and storage location
// after encryption.
type EncryptSecretsResult struct {
	// Location reports the origin or target descriptor of the secrets repository
	// where the updates were applied.
	Location string
	// Secrets lists changed secret identities.
	Secrets []SecretReference
}

// DecryptSecretsResult reports changed secret identities, skipped groups, and
// storage location after decryption.
type DecryptSecretsResult struct {
	// Location reports the origin or target descriptor of the secrets repository
	// where the updates were applied.
	Location string
	// Secrets lists changed secret identities.
	Secrets []SecretReference
	// UnavailableGroups lists groups skipped because no private key was available.
	UnavailableGroups []string
}

// GenerateKeypairResult reports metadata and target file locations for a newly
// generated keypair.
type GenerateKeypairResult struct {
	// PublicKeyLocation reports the secrets repository location where the public key
	// was committed.
	PublicKeyLocation string
	// PrivateKeyLocation reports the private-key repository location where the private
	// key was stored.
	PrivateKeyLocation string
	// Keypair reports public key metadata and private key status.
	Keypair KeypairMetadata
}

// RotateKeypairResult reports metadata, re-encrypted secrets, and file
// locations for a rotated keypair.
type RotateKeypairResult struct {
	// PublicKeyLocation reports the secrets repository location where the new public key
	// and re-encrypted secrets were committed.
	PublicKeyLocation string
	// PrivateKeyLocation reports the private-key repository location where the new
	// private key was stored.
	PrivateKeyLocation string
	// Keypair reports updated public key metadata and private key status.
	Keypair KeypairMetadata
	// Secrets lists re-encrypted secret identities.
	Secrets []SecretReference
}

// StoredSecret reports one stored entry's identity and how its stored value is
// encoded. It never carries the value itself, so a caller can audit the store
// without materializing plaintext.
type StoredSecret struct {
	// Group is the stored key-group name in its stored form.
	Group string
	// Key is the entry name within Group.
	Key string
	// Encrypted reports whether the stored value claims the ciphertext envelope
	// format; a false value marks plaintext left in the store.
	Encrypted bool
	// AlgorithmMismatch reports whether an encrypted value's envelope algorithm
	// differs from the configured cipher, so it could never be decrypted here. It
	// is always false for a plaintext value.
	AlgorithmMismatch bool
}
