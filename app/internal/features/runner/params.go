package runner

import (
	"io"
	"os"
)

// Params configures a single child execution: the merged env to inject plus the
// parameters controlling how Run runs the command. Every field is optional; Run
// normalizes terminal defaults itself via normalizeParams.
type Params struct {
	// Env is the complete, ready-to-inject set of env vars for the child process.
	// It is the effective environment the caller already composed (namespace
	// values, OS overrides, and OS-only keys); Run injects it verbatim.
	Env map[string]string
	// Stdout override where the child's output is written.
	// When nil, os.Stdout is used (normal interactive mode).
	// This is configurable primarily for in-process testing.
	Stdout io.Writer
	// Stderr override where the child's error output is written.
	// When nil, os.Stderr is used (normal interactive mode).
	// This is configurable primarily for in-process testing.
	Stderr io.Writer
}

// normalizeParams applies runner's terminal defaults to Params: a nil Stdout or
// Stderr falls back to the process's os.Stdout/os.Stderr. It mutates Params in
// place so Run can read the effective writers directly.
func normalizeParams(p *Params) {
	if p.Stdout == nil {
		p.Stdout = os.Stdout
	}
	if p.Stderr == nil {
		p.Stderr = os.Stderr
	}
}
