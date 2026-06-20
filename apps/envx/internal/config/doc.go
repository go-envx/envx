// Package config is the CLI resolution pipeline: it meshes command-line flag
// values, ENVX_* environment variables, and the loaded manifest into the
// *engine.Config the engine consumes. Loading the manifest belongs to the
// manifest package; applying terminal defaults belongs to the engine. This
// package owns only the precedence layering in between and imports no CLI
// framework, so it stays reusable by a non-cobra frontend.
package config
