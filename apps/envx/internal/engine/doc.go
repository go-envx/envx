// Package engine owns environment resolution: defaulting + validation of the
// target environment, namespace building, deep-merge, and origin tracking. It
// consumes a plain engine.Config (built by the config package) and imports only
// the pure settings leaf (for settings.Resolved and the terminal DefaultEnv), so
// the "namespace -> merge" pipeline lives entirely behind Build and returns one
// immutable Result. The merge internals are unexported (merge.go) because the
// engine is their only consumer.
package engine
