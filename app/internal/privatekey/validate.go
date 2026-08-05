package privatekey

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// -------------------------------------------------------------------------------------

// validateGroup rejects empty, whitespace, or line-breaking group names so
// lookups match normalized file entries and cannot corrupt env-var names.
func validateGroup(group string) error {
	if strings.TrimSpace(group) == "" {
		return errors.New("private-key group is empty")
	}
	if strings.ContainsRune(group, '=') || strings.IndexFunc(group, unicode.IsSpace) >= 0 {
		return fmt.Errorf("invalid private-key group %q", group)
	}
	return nil
}

// -------------------------------------------------------------------------------------

// validateEntry rejects values that could escape the one-entry key-file format.
func validateEntry(group, privateKey string) error {
	if err := validateGroup(group); err != nil {
		return err
	}
	if privateKey == "" {
		return errors.New("private key is empty")
	}
	if strings.ContainsAny(privateKey, "\r\n") {
		return errors.New("private key contains a line break")
	}
	return nil
}
