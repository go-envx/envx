// Package config is the CLI input boundary and resolution pipeline: it binds the
// flag specs onto cobra commands (the New<X>Flag constructors) and meshes the
// resulting flag values, ENVX_* environment variables, and the loaded manifest
// into the *engine.Config the engine consumes. Loading the manifest belongs to
// the manifest package; applying terminal defaults belongs to the engine. This
// package owns flag registration plus the precedence layering in between.
package config
