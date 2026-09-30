package secrets

// PrivateKeyStatus reports whether usable private-key material is available.
type PrivateKeyStatus string

const (
	// PrivateKeyNotAvailable means no private key was found for the group.
	PrivateKeyNotAvailable PrivateKeyStatus = "not_available"
	// PrivateKeyValid means private key material matches the stored public key.
	PrivateKeyValid PrivateKeyStatus = "valid"
	// PrivateKeyInvalid means private key material is malformed or mismatched.
	PrivateKeyInvalid PrivateKeyStatus = "invalid"
)

// KeypairRecord holds public key data stored for a group.
type KeypairRecord struct {
	Group     string
	PublicKey string
}

// KeypairMetadata reports public key metadata without private-key material.
type KeypairMetadata struct {
	// Group is the key-group being reported.
	Group string
	// PublicKey is the stored public encryption key.
	PublicKey string
	// PrivateKeyStatus reports the safe private-key availability state.
	PrivateKeyStatus PrivateKeyStatus
}
