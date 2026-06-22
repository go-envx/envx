// Package config is the CLI input boundary and resolution pipeline: it binds the
// settings specs onto cobra commands (the BindString/BindBool/BindPersistentString
// helpers) and meshes the resulting flag values, ENVX_* environment variables, and
// the loaded manifest into the *engine.Config the engine consumes. Loading the
// manifest belongs to the manifest package; the setting catalog and terminal
// defaults belong to the settings package; applying those defaults belongs to the
// engine. This package owns flag binding plus the precedence layering in between.
package config
