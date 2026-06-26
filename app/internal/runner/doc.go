// Package runner executes child processes with environment injection, signal
// forwarding, and exit-code propagation. It is decoupled from any concrete
// error type: callers supply a mapper that turns a non-zero child exit code
// into the error they want surfaced.
package runner
