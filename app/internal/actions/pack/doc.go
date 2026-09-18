// Package pack implements "envx pack", which copies an environment-scoped subset
// of the workspace into an output directory that runs through the ordinary
// `envx run --config` pipeline. It resolves the workspace layout, delegates file
// selection and copying to internal/pack, and renders a summary of what it wrote.
package pack
