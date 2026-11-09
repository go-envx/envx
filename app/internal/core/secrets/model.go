package secrets

// OriginStore is the provenance of a private key read from local key storage
// rather than an overriding environment variable.
const OriginStore = "store"

// Group is one group's public key and stored values.
type Group struct {
	// Name is the group name as stored; lookups ignore case.
	Name string
	// PublicKey encrypts new values; empty when the group has no keypair.
	PublicKey string
	// Values maps keys to ciphertext envelopes or legacy plaintext, never
	// decrypted values.
	Values map[string]string
}

// Snapshot is a group read at one revision.
type Snapshot struct {
	// Group is the group as read.
	Group Group
	// Revision is an opaque token a commit must match to replace the group.
	Revision string
	// Exists is false for a group with neither a public key nor stored values.
	Exists bool
}

// PrivateKey is transient private-key material and its provenance.
type PrivateKey struct {
	// Value is the private key; it must not be logged or reported.
	Value string
	// Origin names the key's source, such as OriginStore or an environment
	// variable.
	Origin string
}
