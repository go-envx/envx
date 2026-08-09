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
)

// -------------------------------------------------------------------------------------

// AlgorithmOptions is the closed set of algorithm-specific options accepted
// by New. Each supported algorithm provides its own concrete options type.
type AlgorithmOptions interface {
	algorithmOptions()
}

// -------------------------------------------------------------------------------------

// Params selects a cipher implementation and supplies its algorithm-specific options.
type Params struct {
	// Algorithm identifies the cipher implementation to construct.
	Algorithm Algorithm
	// Options contains the options for Algorithm. A nil value selects the
	// algorithm's default options.
	Options AlgorithmOptions
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

// Cipher is the contract for key generation, secret encryption, and envelope
// algorithm metadata. Key and ciphertext formats are implementation details,
// allowing callers to select an algorithm without branching on its
// representation.
type Cipher interface {
	// Algorithm identifies the algorithm produced by the cipher.
	Algorithm() Algorithm
	// Keypair generates a new public/private keypair.
	Keypair() (Keypair, error)
	// ValidateKeypair checks key format and public/private correspondence.
	ValidateKeypair(publicKey, privateKey string) error
	// Encrypt encrypts plaintext for publicKey and returns native ciphertext bytes.
	Encrypt(plaintext, publicKey string) ([]byte, error)
	// Decrypt decrypts native ciphertext bytes with privateKey.
	Decrypt(ciphertext []byte, privateKey string) (string, error)
}

// -------------------------------------------------------------------------------------

// New constructs the cipher implementation selected by params. The selector is
// explicit so each algorithm can own its option type.
func New(params Params) (Cipher, error) {
	switch params.Algorithm {

	// Validate Age algorithm options and construct the cipher.
	case Age:
		options := params.Options
		if options == nil {
			options = AgeOptions{}
		}
		ageOptions, ok := options.(AgeOptions)
		if !ok {
			return nil, fmt.Errorf("options for %q must be cipher.AgeOptions", Age)
		}
		return newAgeCipher(ageOptions)

	// Validate NaCl Box algorithm options and construct the cipher.
	case NaClBox:
		options := params.Options
		if options == nil {
			options = NaClBoxOptions{}
		}
		naclBoxOptions, ok := options.(NaClBoxOptions)
		if !ok {
			return nil, fmt.Errorf(
				"options for %q must be cipher.NaClBoxOptions", NaClBox,
			)
		}
		return newNaClBoxCipher(naclBoxOptions)

	// Reject any unsupported algorithm selection.
	default:
		return nil, fmt.Errorf("unsupported cipher algorithm %q", params.Algorithm)

	}
}
