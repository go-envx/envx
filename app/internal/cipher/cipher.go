package cipher

import "fmt"

// -------------------------------------------------------------------------------------

// Algorithm identifies an encryption algorithm supported by the cipher factory.
type Algorithm string

const (
	// Age identifies age encryption using X25519 identities.
	Age Algorithm = "age"
	// NaClBox identifies NaCl sealed-box encryption using Curve25519.
	NaClBox Algorithm = "nacl-box"

	// DefaultAlgorithm is the algorithm used by envx when no selection is
	// supplied by a future caller.
	DefaultAlgorithm = Age
)

// -------------------------------------------------------------------------------------

// AlgorithmOptions is the closed set of algorithm-specific options accepted
// by New. Each supported algorithm provides its own concrete options type.
type AlgorithmOptions interface {
	algorithmOptions()
}

// -------------------------------------------------------------------------------------

// Keypair contains the opaque public and private key strings produced by a
// cipher. PrivateKey is transient material and must not be included in status
// or mutation results.
type Keypair struct {
	// PublicKey is the key used to encrypt values.
	PublicKey string
	// PrivateKey is the key used to decrypt values.
	PrivateKey string
}

// -------------------------------------------------------------------------------------

// Cipher is the algorithm-neutral contract for key generation and secret
// encryption. Key and ciphertext formats are implementation details, allowing
// callers to select an algorithm without branching on its representation.
type Cipher interface {
	// Keypair generates a new public/private keypair.
	Keypair() (Keypair, error)
	// Encrypt encrypts plaintext for publicKey and returns native ciphertext bytes.
	Encrypt(plaintext, publicKey string) ([]byte, error)
	// Decrypt decrypts native ciphertext bytes with privateKey.
	Decrypt(ciphertext []byte, privateKey string) (string, error)
}

// -------------------------------------------------------------------------------------

// New constructs the cipher implementation selected by algorithm and options.
// The selector is explicit so each algorithm can own its option type.
func New(algorithm Algorithm, options AlgorithmOptions) (Cipher, error) {
	switch algorithm {
	case Age:
		ageOptions, ok := options.(AgeOptions)
		if !ok {
			return nil, fmt.Errorf("options for %q must be cipher.AgeOptions", Age)
		}
		return newAgeCipher(ageOptions)
	case NaClBox:
		naclBoxOptions, ok := options.(NaClBoxOptions)
		if !ok {
			return nil, fmt.Errorf(
				"options for %q must be cipher.NaClBoxOptions", NaClBox,
			)
		}
		return newNaClBoxCipher(naclBoxOptions)
	default:
		return nil, fmt.Errorf("unsupported cipher algorithm %q", algorithm)
	}
}
