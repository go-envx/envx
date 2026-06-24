// Package config is the resolution pipeline: it meshes flag values, ENVX_*
// environment variables, and the loaded manifest into the *envmerge.Config that
// envmerge consumes, applying the precedence flag > ENVX_* > project > global. It is
// framework-agnostic — no cobra — reading flag state only through the small
// FlagSet interface. Loading the manifest belongs to the manifest package; the
// setting catalog and terminal defaults belong to the schema package; binding
// specs onto cobra commands belongs to the flags package; and applying terminal
// defaults belongs to envmerge. This package owns only the precedence layering.
package config
