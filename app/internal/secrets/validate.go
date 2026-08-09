package secrets

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// -------------------------------------------------------------------------------------

// normalizeGroupName validates a key-group name and returns its canonical form.
func normalizeGroupName(group string) (string, error) {
	if strings.TrimSpace(group) == "" {
		return "", errors.New("secret group is empty")
	}
	if strings.IndexFunc(group, unicode.IsSpace) >= 0 ||
		strings.ContainsAny(group, "/\r\n=") {
		return "", fmt.Errorf("invalid secret group %q", group)
	}
	return strings.ToLower(group), nil
}

// -------------------------------------------------------------------------------------

// validateSecretKey validates the exact key identifier accepted by references
// and the document store.
func validateSecretKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("secret key is empty")
	}
	if strings.ContainsAny(key, "/\r\n") {
		return fmt.Errorf("invalid secret key %q", key)
	}
	return nil
}
