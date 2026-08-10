package privatekey

import (
	"bytes"
	"errors"
	"testing"
)

// destinationErrorWriter returns one configured error for every write.
type destinationErrorWriter struct {
	// err is returned from Write.
	err error
}

// Write returns the configured destination error.
func (w destinationErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

// TestWriterDestinationWritesNormalizedEntry verifies the exact handoff format.
func TestWriterDestinationWritesNormalizedEntry(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := NewWriterDestination(&output).Write(
		"Production", "private-value",
	); err != nil {
		t.Fatalf("Write(): %v", err)
	}
	if got := output.String(); got != "PRODUCTION=private-value\n" {
		t.Errorf("writer output = %q", got)
	}
}

// TestWriterDestinationRejectsNilWriter verifies a missing writer fails clearly.
func TestWriterDestinationRejectsNilWriter(t *testing.T) {
	t.Parallel()

	err := NewWriterDestination(nil).Write("production", "private-value")
	if err == nil {
		t.Fatal("Write() accepted a nil writer")
	}
	if got := err.Error(); got != "private-key writer is nil" {
		t.Errorf("Write() error = %q, want nil-writer error", got)
	}
}

// TestWriterDestinationWrapsWriterError verifies errors from the handoff writer
// remain discoverable by callers.
func TestWriterDestinationWrapsWriterError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("writer failed")
	err := NewWriterDestination(destinationErrorWriter{err: sentinel}).Write(
		"production", "private-value",
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Write() error = %v, want wrapped writer error", err)
	}
}
