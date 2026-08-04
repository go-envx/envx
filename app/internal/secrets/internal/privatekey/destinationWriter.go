package privatekey

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// -------------------------------------------------------------------------------------

// writerDestination hands private keys to an explicitly selected writer.
type writerDestination struct {
	// writer receives one NAME=value entry.
	writer io.Writer
}

// -------------------------------------------------------------------------------------

// NewWriterDestination creates a destination that writes one explicit handoff.
func NewWriterDestination(writer io.Writer) Destination {
	return writerDestination{writer: writer}
}

// -------------------------------------------------------------------------------------

// Write sends one NAME=value entry to the explicit writer.
func (d writerDestination) Write(group, privateKey string) error {
	if err := validateEntry(group, privateKey); err != nil {
		return err
	}
	if d.writer == nil {
		return errors.New("private-key writer is nil")
	}
	if _, err := fmt.Fprintf(
		d.writer, "%s=%s\n", strings.ToUpper(group), privateKey,
	); err != nil {
		return fmt.Errorf("writing private key: %w", err)
	}
	return nil
}
