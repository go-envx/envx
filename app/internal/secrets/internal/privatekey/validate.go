package privatekey

import (
	"errors"
	"strings"
)

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
