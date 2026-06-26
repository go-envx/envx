// Package config is the resolution pipeline: it meshes flag values, ENVX_*
// environment variables, and the loaded manifest into a resolved *envmerge.Params,
// applying the precedence flag > ENVX_* > project > global. It is framework-agnostic
// — no cobra — reading flag state only through the small FlagSet interface. It owns
// only the precedence layering and deliberately leaves terminal defaults (such as
// the first-declared environment) unset.
package config
