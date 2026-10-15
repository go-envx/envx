package cipher

import (
	"fmt"
	"maps"
	"sync"

	"github.com/go-envx/envx/app/internal/shared/value"
)

// Provider is the contract for key generation, secret encryption, and envelope
// algorithm metadata implemented by cryptographic engines. Key and ciphertext formats
// are implementation details, allowing callers to select an algorithm without
// branching on its representation.
type Provider interface {
	// Algorithm identifies the algorithm produced by the provider.
	Algorithm() string
	// Keypair generates a new public/private keypair of opaque key strings.
	Keypair() (value.Keypair, error)
	// ValidateKeypair checks key format and public/private correspondence.
	ValidateKeypair(publicKey, privateKey string) error
	// Encrypt encrypts plaintext for publicKey and returns native ciphertext bytes.
	Encrypt(plaintext, publicKey string) ([]byte, error)
	// Decrypt decrypts native ciphertext bytes with privateKey.
	Decrypt(ciphertext []byte, privateKey string) (string, error)
}

// Params configures the cipher client.
type Params struct {
	// Providers is the initial set of cryptographic providers to register.
	Providers []Provider
}

// Client manages cryptographic providers and routes cipher operations.
type Client struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// New constructs a cipher Client initialized with any provided providers.
func New(params Params) (*Client, error) {
	client := &Client{
		providers: make(map[string]Provider),
	}
	for _, provider := range params.Providers {
		if err := client.Register(provider); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// Register registers a cryptographic provider under its algorithm identifier.
// An error is returned if a provider with the same algorithm is already registered.
func (c *Client) Register(provider Provider) error {
	if provider == nil {
		return ErrNilProvider
	}
	alg := provider.Algorithm()
	if alg == "" {
		return ErrEmptyAlgorithm
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.providers[alg]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateAlgorithm, alg)
	}
	c.providers[alg] = provider
	return nil
}

// For returns the cryptographic provider registered for the given algorithm name.
// Returns an error if no provider is registered for that algorithm.
func (c *Client) For(algorithm string) (Provider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	provider, ok := c.providers[algorithm]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownAlgorithm, algorithm)
	}
	return provider, nil
}

// All returns a shallow copy of all registered providers keyed by algorithm name.
func (c *Client) All() map[string]Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return maps.Clone(c.providers)
}
