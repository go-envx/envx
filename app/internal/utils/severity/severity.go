package severity

import (
	"errors"
	"strings"
)

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

// ErrInvalidLevel reports an unrecognized severity level string.
var ErrInvalidLevel = errors.New("invalid severity level")

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

// Parse converts a string ("none", "ok", "warn", "warning", "error", or "off")
// into a Level. "off" and "none" map to None; "warn" and "warning" map to Warn.
func Parse(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none", "off":
		return None, nil
	case "ok":
		return OK, nil
	case "warn", "warning":
		return Warn, nil
	case "error":
		return Error, nil
	default:
		return None, ErrInvalidLevel
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
