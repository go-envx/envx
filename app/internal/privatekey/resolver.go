package privatekey

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-envx/envx/app/pkg/file"
)

const (
	privateKeyEnv = "ENVX_PRIVATE_KEY"
)

// -------------------------------------------------------------------------------------

// PrivateKey contains transient private-key material and its lookup provenance.
type PrivateKey struct {
	// Value is the private key used for one operation.
	Value string
	// Origin identifies the lookup location that supplied Value without including Value.
	Origin string
}

// -------------------------------------------------------------------------------------

// ErrNotAvailable indicates that no private key is available for a group.
var ErrNotAvailable = errors.New("private key not available")

// -------------------------------------------------------------------------------------

// Resolver resolves private-key material for one group.
type Resolver interface {
	// Resolve returns a private key and its lookup provenance.
	Resolve(group string) (PrivateKey, error)
}

// -------------------------------------------------------------------------------------

// ResolverOptions configures automatic environment-then-file lookup.
type ResolverOptions struct {
	// KeysPath is the optional local private-key file to read after environment sources.
	KeysPath string
	// LookupEnv reads an environment variable and reports whether it is present.
	LookupEnv func(string) (string, bool)
}

// -------------------------------------------------------------------------------------

// resolver implements Resolver with deterministic environment and file precedence.
type resolver struct {
	// keysPath is the configured local private-key file.
	keysPath string
	// lookupEnv reads environment variables without requiring process-global
	// state in tests.
	lookupEnv func(string) (string, bool)
}

// -------------------------------------------------------------------------------------

// NewResolver creates an automatic environment-then-file private-key resolver.
func NewResolver(options ResolverOptions) Resolver {
	lookupEnv := options.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	return resolver{keysPath: options.KeysPath, lookupEnv: lookupEnv}
}

// -------------------------------------------------------------------------------------

// Resolve returns the first available private key for group. A present but empty
// or malformed higher-priority input is an error and does not fall through.
func (r resolver) Resolve(group string) (PrivateKey, error) {
	if err := validateGroup(group); err != nil {
		return PrivateKey{}, err
	}

	specificName := privateKeyEnv + "_" + strings.ToUpper(group)
	if value, present := r.lookupEnv(specificName); present {
		if value == "" {
			return PrivateKey{}, fmt.Errorf("environment variable %s is empty", specificName)
		}
		return PrivateKey{Value: value, Origin: specificName}, nil
	}

	if value, present := r.lookupEnv(privateKeyEnv); present {
		if value == "" {
			return PrivateKey{}, fmt.Errorf("environment variable %s is empty", privateKeyEnv)
		}
		parsed, err := parseKeyFile(value, privateKeyEnv)
		if err != nil {
			return PrivateKey{}, err
		}
		if key, found := parsed.lookup(group); found {
			return PrivateKey{Value: key, Origin: privateKeyEnv}, nil
		}
	}

	if r.keysPath == "" {
		return PrivateKey{}, unavailable(group)
	}
	data, err := file.Read(r.keysPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PrivateKey{}, unavailable(group)
		}
		return PrivateKey{}, fmt.Errorf("reading private keys %s: %w", r.keysPath, err)
	}

	parsed, err := parseKeyFile(string(data), r.keysPath)
	if err != nil {
		return PrivateKey{}, err
	}
	key, found := parsed.lookup(group)
	if !found {
		return PrivateKey{}, unavailable(group)
	}
	return PrivateKey{Value: key, Origin: r.keysPath}, nil
}

// -------------------------------------------------------------------------------------

// unavailable returns a typed error for a missing group key.
func unavailable(group string) error {
	return fmt.Errorf("%w for group %q", ErrNotAvailable, group)
}
