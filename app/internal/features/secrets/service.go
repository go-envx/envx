package secrets

import (
	"errors"

	"github.com/go-envx/envx/app/internal/features/privatekey"
)

// PrivateKeyService defines the contract secrets consumes from the private key domain.
type PrivateKeyService interface {
	Location() string
	Resolve(group string) (privatekey.PrivateKey, error)
	Set(group, privateKey string) error
}

// CipherClient defines the cryptographic operations secrets consumes.
type CipherClient interface {
	Algorithm() string
	Keypair() (publicKey, privateKey string, err error)
	ValidateKeypair(publicKey, privateKey string) error
	Encrypt(plaintext, publicKey string) ([]byte, error)
	Decrypt(ciphertext []byte, privateKey string) (string, error)
}

// Repository defines storage operations consumed by Service.
type Repository interface {
	// Location reports the origin or target descriptor of the repository
	// (e.g. file path, remote URI, or storage identifier).
	Location() string

	// GetPublicKey returns the public key for a group.
	GetPublicKey(group string) (string, bool, error)
	// SetPublicKey sets or updates the public key for a group.
	SetPublicKey(group, publicKey string) error
	// ListKeypairs returns all configured keypair records.
	ListKeypairs() ([]KeypairRecord, error)

	// GetSecret retrieves an encrypted secret record.
	GetSecret(group, key string) (SecretRecord, bool, error)
	// SetSecret stores an encrypted secret record.
	SetSecret(record SecretRecord) error
	// DeleteSecret removes a secret record.
	DeleteSecret(group, key string) error
	// ListSecrets returns all stored secret records.
	ListSecrets() ([]SecretRecord, error)
}

// BatchRepository is an optional repository capability for atomic bulk updates.
type BatchRepository interface {
	SetSecrets(records []SecretRecord) error
}

// ServiceParams provides dependencies to the secrets domain service.
type ServiceParams struct {
	Repository        Repository
	Cipher            CipherClient
	PrivateKeyService PrivateKeyService
	// CiphertextPrefix optionally configures the storage envelope marker.
	CiphertextPrefix string
}

// Service coordinates secret CRUD, keypair lifecycle, and reference evaluation.
type Service struct {
	params ServiceParams
	codec  *envelopeCodec
}

// NewService constructs a secrets domain service.
func NewService(params ServiceParams) (*Service, error) {
	if params.Repository == nil {
		return nil, errors.New("repository is required")
	}
	if params.Cipher == nil {
		return nil, errors.New("cipher is required")
	}
	if params.PrivateKeyService == nil {
		return nil, errors.New("private-key service is required")
	}
	codec := newEnvelopeCodec(envelopeParams{
		Prefix: params.CiphertextPrefix,
	})
	return &Service{
		params: params,
		codec:  codec,
	}, nil
}

// Location reports the origin or target descriptor of the underlying
// secrets repository (e.g. file path, remote URI, or storage identifier).
func (s *Service) Location() string {
	return s.params.Repository.Location()
}

// KeysLocation reports the storage location of the underlying private-key
// repository, or empty string if no private-key service is configured.
func (s *Service) KeysLocation() string {
	if s.params.PrivateKeyService == nil {
		return ""
	}
	return s.params.PrivateKeyService.Location()
}
