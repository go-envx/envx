// Package printer composes style primitives into semantic terminal output. A
// Printer owns the standard output and error streams, detects each stream's color
// capability once, and exposes intent-named operations (LogMessage, LogWarning,
// LogError, WriteTable, WriteJSON) so actions render consistently without wiring
// styles or color flags themselves.
package printer
