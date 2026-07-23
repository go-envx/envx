package runner

import (
	"io"
	"os"
)

// -------------------------------------------------------------------------------------

// Params configures a single child execution: the merged env to inject plus the
// parameters controlling how Run runs the command. Every field is optional; Run
// normalizes terminal defaults itself via normalizeParams.
type Params struct {
	// Env is the merged set of env vars to inject into the child process.
	Env map[string]string
	// Overload controls env-var precedence:
	//   false (default): existing OS env vars take priority over file values.
	//   true:            file values override existing OS env vars.
	Overload bool
	// Stdout override where the child's output is written.
	// When nil, os.Stdout is used (normal interactive mode).
	// This is configurable primarily for in-process testing.
	Stdout io.Writer
	// Stderr override where the child's error output is written.
	// When nil, os.Stderr is used (normal interactive mode).
	// This is configurable primarily for in-process testing.
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------

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
