package runner

import (
	"bytes"
	"os"
	"testing"
)

// TestNormalizeParamsDefaultsStreams verifies that nil Stdout/Stderr fall back to
// the process's standard streams.
func TestNormalizeParamsDefaultsStreams(t *testing.T) {
	t.Parallel()

	p := Params{}
	normalizeParams(&p)

	if p.Stdout != os.Stdout {
		t.Errorf("Stdout = %v, want os.Stdout", p.Stdout)
	}
	if p.Stderr != os.Stderr {
		t.Errorf("Stderr = %v, want os.Stderr", p.Stderr)
	}
}

// TestNormalizeParamsPreservesStreams verifies that explicit Stdout/Stderr writers
// are left untouched.
func TestNormalizeParamsPreservesStreams(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	p := Params{Stdout: &out, Stderr: &errBuf}
	normalizeParams(&p)

	if p.Stdout != &out {
		t.Error("Stdout was replaced, want the provided writer")
	}
	if p.Stderr != &errBuf {
		t.Error("Stderr was replaced, want the provided writer")
	}
}
