// Package envmerge owns environment resolution: defaulting + validation of the
// target environment, namespace building, deep-merge, and origin tracking. It
// consumes a plain envmerge.Config (built by the config package), owns the resolved
// envmerge.Settings the merge reads, and applies the terminal default (the first
// declared environment) itself, so the "namespace -> merge" pipeline lives entirely
// behind Build and returns one immutable Result. The merge internals are unexported
// (merge.go) because envmerge is their only consumer.
package envmerge
