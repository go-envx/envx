// Package runner handles executing child processes with environment injection,
// signal forwarding, and exit-code propagation. It is the final stage of the
// "envx run" pipeline: after environment resolution, this package spawns the
// child command with the assembled environment. It is domain-agnostic — it
// knows nothing about manifests or merging.
package runner
