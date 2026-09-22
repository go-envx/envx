// Package style provides capability-gated ANSI styling primitives. A Styler
// wraps strings in color or weight codes only when enabled, so callers can build
// styled output without branching on terminal support themselves. It performs no
// I/O and detects no capabilities; the printer package owns stream detection and
// constructs Stylers accordingly.
package style
