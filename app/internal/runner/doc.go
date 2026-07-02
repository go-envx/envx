// Package runner executes child processes with environment injection, signal
// forwarding, and exit-code propagation. It stays transparent — relaying
// received signals to the child and mirroring its exit status rather than
// force-killing it — and surfaces a non-zero or signal-terminated exit as an
// *exitcode.Error the process boundary can propagate.
package runner
