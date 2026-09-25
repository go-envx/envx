package privatekey

import (
	"fmt"
	"strings"
	"unicode"
)

// PrivateKey contains transient private-key material and its lookup provenance.
type PrivateKey struct {
	// Value is the private key used for one operation.
	Value string
	// Origin identifies the lookup location that supplied Value without including Value.
	Origin string
}

// ValidateGroup rejects empty, whitespace, or line-breaking group names so
// lookups match normalized file entries and cannot corrupt env-var names.
func ValidateGroup(group string) error {
	if strings.TrimSpace(group) == "" {
		return ErrEmptyGroup
	}
	if strings.ContainsRune(group, '=') || strings.IndexFunc(group, unicode.IsSpace) >= 0 {
		return fmt.Errorf("%w %q", ErrInvalidGroup, group)
	}
	return nil
}

// ValidateEntry rejects values that could escape the one-entry key-file format.
func ValidateEntry(group, privateKey string) error {
	if err := ValidateGroup(group); err != nil {
		return err
	}
	if privateKey == "" {
		return ErrEmptyKey
	}
	if strings.ContainsAny(privateKey, "\r\n") {
		return ErrKeyHasLineBreak
	}
	return nil
}
