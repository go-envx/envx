// Package envmerge owns environment resolution: defaulting + validation of the
// target environment, namespace building, per-file flattening with layered
// overlay merge, and origin tracking. It
// consumes a plain envmerge.Params, owns the resolved envmerge.Settings the merge
// reads, and applies the terminal default (the first declared environment) itself,
// so the "namespace -> merge" pipeline lives entirely behind a Manager whose
// operations (Get, Explain, Diff, Materialize) each start from fresh namespace and
// resolver state. The generic map helpers are unexported because envmerge is
// their only consumer.
package envmerge
