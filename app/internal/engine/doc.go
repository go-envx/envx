// Package engine owns environment resolution: defaulting + validation of the
// target environment, namespace building, deep-merge, and origin tracking. It
// consumes a plain engine.Config (built by the config package), owns the resolved
// engine.Settings the merge reads, and applies the terminal default (the first
// declared environment) itself, so the "namespace -> merge" pipeline lives entirely
// behind Build and returns one immutable Result. The merge internals are unexported
// (merge.go) because the engine is their only consumer.
package engine
