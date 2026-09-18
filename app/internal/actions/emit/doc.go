// Package emit implements "envx emit", which fully resolves and decrypts a
// single project environment and renders it to a delivery target — dotenv, JSON,
// or split Kubernetes Secret/ConfigMap manifests. It reveals every value through
// the diagnostic reveal path, fails closed when any value is unresolved so no
// partial output escapes, and delegates formatting to internal/emit.
package emit
