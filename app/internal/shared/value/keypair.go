package value

// Keypair is an opaque public/private key pair produced by a cipher. PrivateKey
// is transient material and must not appear in status or mutation results.
type Keypair struct {
	// PublicKey is the key used to encrypt values.
	PublicKey string
	// PrivateKey is the key used to decrypt values.
	PrivateKey string
}
