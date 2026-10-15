package severity

// Level represents an ordinal severity level.
type Level int

const (
	// None indicates no severity classification or unstyled output.
	None Level = iota
	// OK marks a successful outcome with no concerns.
	OK
	// Warn marks a non-fatal condition, warning, or recoverable concern.
	Warn
	// Error marks a failing condition, invalid state, or fatal problem.
	Error
)

// String returns the canonical lowercase name for the severity level.
func (l Level) String() string {
	switch l {
	case None:
		return "none"
	case OK:
		return "ok"
	case Warn:
		return "warn"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

// DisplayString returns the full presentation label (e.g. "warning" for Warn).
func (l Level) DisplayString() string {
	switch l {
	case Warn:
		return "warning"
	default:
		return l.String()
	}
}

// Rank returns the numeric rank for ordering severities (None: 0, OK: 1,
// Warn: 2, Error: 3).
func (l Level) Rank() int {
	return int(l)
}

// Worst returns the higher severity between a and b.
func Worst(a, b Level) Level {
	if a.Rank() >= b.Rank() {
		return a
	}
	return b
}
