// Package config is the resolution pipeline: it meshes the user's optional input
// values, ENVX_* environment variables, and the loaded manifest into a single
// resolved *Resolved, applying the precedence explicit > ENVX_* > project >
// global. One Resolve call loads the manifest once and returns the aggregate
// (the envmerge.Params plus the resolved overload knob); actions read the fields
// they need. It is framework-agnostic — no cobra — taking presence as an optional
// (non-nil) value rather than any flag-set handle, so the same pipeline serves a
// future API. It owns only the precedence layering and deliberately leaves
// terminal defaults (such as the first-declared environment) unset.
package config
