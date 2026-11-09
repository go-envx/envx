package diagnostic

import (
	"strings"

	"github.com/go-envx/envx/app/internal/lib/severity"
)

// Resolve converts configured overrides, keyed by case-insensitive code (for
// example "secret_is_not_referenced") with an off, warn, or error value, into
// levels keyed by canonical code. Off resolves to severity.None. Empty input
// yields a nil map, meaning every code keeps its default.
func Resolve(overrides map[string]string) (map[Code]severity.Level, error) {
	if len(overrides) == 0 {
		return nil, nil
	}
	out := make(map[Code]severity.Level, len(overrides))
	for key, raw := range overrides {
		code := Code(strings.ToUpper(key))
		if _, ok := Lookup(code); !ok {
			return nil, &UnknownCodeError{Key: key}
		}
		level, err := parseLevel(raw)
		if err != nil {
			return nil, err
		}
		out[code] = level
	}
	return out, nil
}

func parseLevel(raw string) (severity.Level, error) {
	switch raw {
	case "off":
		return severity.None, nil
	case "warn":
		return severity.Warn, nil
	case "error":
		return severity.Error, nil
	default:
		return severity.None, &InvalidSeverityError{Value: raw}
	}
}

// UnknownCodeError reports an override key that is not a reportable code.
type UnknownCodeError struct {
	Key string
}

func (e *UnknownCodeError) Error() string {
	return "unknown validate check " + e.Key
}

// InvalidSeverityError reports an override value that is not off, warn, or error.
type InvalidSeverityError struct {
	Value string
}

func (e *InvalidSeverityError) Error() string {
	return "invalid severity " + e.Value + " (want off, warn, or error)"
}
