// Package emit renders a fully resolved environment to a delivery target:
// dotenv, JSON, or a Kubernetes Secret/ConfigMap split by whether each value is
// secret-derived. It is the pure formatting core behind "envx emit": callers
// hand its Service already-resolved key/value entries and it writes the chosen
// format to the Service's writer, performing no resolution, decryption, or
// environment I/O of its own.
package emit
