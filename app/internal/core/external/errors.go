package external

import (
	"errors"
	"strings"
)

// maxStderrSummary bounds the provider diagnostic quoted in errors.
const maxStderrSummary = 200

var (
	// ErrNotReference reports a value that does not use an external scheme.
	ErrNotReference = errors.New("not an external reference")
	// ErrInvalidReference reports a malformed external reference.
	ErrInvalidReference = errors.New("invalid external reference")
	// ErrProviderUnavailable reports a provider whose CLI or credentials are
	// missing, or that timed out earlier in the operation.
	ErrProviderUnavailable = errors.New("external provider unavailable")
	// ErrReferenceUnresolved reports a provider that returned no value.
	ErrReferenceUnresolved = errors.New("external reference unresolved")
)

// SummarizeStderr returns the first line of a provider's stderr, truncated,
// for quoting in errors.
func SummarizeStderr(stderr string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
	line = strings.TrimSpace(line)
	if runes := []rune(line); len(runes) > maxStderrSummary {
		return string(runes[:maxStderrSummary]) + "..."
	}
	if line == "" {
		return "no diagnostic output"
	}
	return line
}
