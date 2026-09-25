package privatekey

import (
	"errors"
	"fmt"
	"strings"
)

const (
	privateKeyEnv = "ENVX_PRIVATE_KEY"
)

// Repository defines persistent storage operations consumed by Service.
type Repository interface {
	// Origin returns the provenance identifier for this repository.
	Origin() string
	// GetPrivateKey retrieves the stored key for a group, or reports false if absent.
	GetPrivateKey(group string) (key string, found bool, err error)
	// SetPrivateKey persists a private key for a group.
	SetPrivateKey(group, privateKey string) error
}

// ServiceParams provides dependencies to the private key domain service.
type ServiceParams struct {
	// Repository provides persistent key storage. Optional; nil skips repository lookup.
	Repository Repository
	// LookupEnv queries environment variables. Required.
	LookupEnv func(string) (string, bool)
}

// Service coordinates private key resolution and persistence across
// environment variables and storage.
type Service struct {
	params ServiceParams
}

// NewService constructs a private key domain service.
func NewService(params ServiceParams) (*Service, error) {
	if params.LookupEnv == nil {
		return nil, errors.New("lookupEnv is required")
	}
	return &Service{params: params}, nil
}

// Resolve returns the first available private key for a group across env vars
// and repository.
func (s *Service) Resolve(group string) (PrivateKey, error) {
	if err := ValidateGroup(group); err != nil {
		return PrivateKey{}, err
	}

	specificName := privateKeyEnv + "_" + strings.ToUpper(group)
	if value, present := s.params.LookupEnv(specificName); present {
		if value == "" {
			return PrivateKey{}, fmt.Errorf(
				"%w: environment variable %s is empty", ErrInvalidKey, specificName,
			)
		}
		return PrivateKey{Value: value, Origin: specificName}, nil
	}

	if value, present := s.params.LookupEnv(privateKeyEnv); present {
		if value == "" {
			return PrivateKey{}, fmt.Errorf(
				"%w: environment variable %s is empty", ErrInvalidKey, privateKeyEnv,
			)
		}
		parsed, err := s.parseKeyEnv(value)
		if err != nil {
			return PrivateKey{}, fmt.Errorf("%w: %w", ErrInvalidKey, err)
		}
		if key, found := parsed[strings.ToUpper(group)]; found {
			return PrivateKey{Value: key, Origin: privateKeyEnv}, nil
		}
	}

	if s.params.Repository == nil {
		return PrivateKey{}, s.unavailable(group)
	}

	key, found, err := s.params.Repository.GetPrivateKey(group)
	if err != nil {
		return PrivateKey{}, err
	}
	if !found {
		return PrivateKey{}, s.unavailable(group)
	}

	return PrivateKey{Value: key, Origin: s.params.Repository.Origin()}, nil
}

// Set validates and persists a private key for a group via the repository.
func (s *Service) Set(group, privateKey string) error {
	if err := ValidateEntry(group, privateKey); err != nil {
		return err
	}
	if s.params.Repository == nil {
		return errors.New("private key repository is nil")
	}
	return s.params.Repository.SetPrivateKey(group, privateKey)
}

// parseKeyEnv parses NAME=value lines from ENVX_PRIVATE_KEY content, rejecting
// malformed, duplicate, and empty entries.
func (s *Service) parseKeyEnv(content string) (map[string]string, error) {
	lines := strings.Split(content, "\n")
	result := make(map[string]string)
	for index, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		name, value, ok := strings.Cut(line, "=")
		name = strings.ToUpper(strings.TrimSpace(name))
		if !ok || name == "" {
			return nil, fmt.Errorf(
				"malformed private-key entry in %s at line %d", privateKeyEnv, index+1,
			)
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf(
				"duplicate private-key group %q in %s", name, privateKeyEnv,
			)
		}
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf(
				"private key for group %q is empty in %s", name, privateKeyEnv,
			)
		}
		result[name] = value
	}
	return result, nil
}

// unavailable returns a typed error for a missing group key.
func (s *Service) unavailable(group string) error {
	return fmt.Errorf("%w for group %q", ErrNotAvailable, group)
}
