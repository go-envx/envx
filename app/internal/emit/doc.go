// Package emit renders a fully resolved environment to a delivery target:
// dotenv, JSON, or a Kubernetes Secret/ConfigMap split by whether each value is
// secret-derived. It is the pure formatting core behind "envx emit": callers
// hand it already-resolved key/value entries and it writes the chosen format to
// a writer, performing no resolution, decryption, or environment I/O of its own.
package emit
