package main

import (
	"bytes"
	"errors"
	"testing"
)

// TestReportErrorUsesPrinter verifies fatal errors use the shared severity
// label and the configured error stream.
func TestReportErrorUsesPrinter(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	err := reportError(&stderr, errors.New("missing public key"))
	if err != nil {
		t.Fatalf("reportError(): %v", err)
	}
	if got, want := stderr.String(), "ERROR: missing public key\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}
