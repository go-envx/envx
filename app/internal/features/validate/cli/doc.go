// Package cli implements "envx validate".
//
// It is the thin Cobra adapter over the internal/features/validate engine: it
// resolves every project in the workspace, runs the workspace-wide diagnosis,
// renders the findings, and returns a non-zero exit when the graded report
// fails.
package cli
