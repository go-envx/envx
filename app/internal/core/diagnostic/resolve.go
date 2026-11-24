package diagnostic

import (
	"regexp"
	"strings"

	"github.com/go-envx/envx/app/internal/lib/severity"
)

var camelCaseKey = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[A-Z][a-z0-9]*)*$`)

// ConfigKey returns the camelCase envx.yaml severity key for code, such as
// "secretIsNotEncrypted" for SECRET_IS_NOT_ENCRYPTED.
func ConfigKey(code Code) string {
	words := strings.Split(strings.ToLower(string(code)), "_")
	for i := 1; i < len(words); i++ {
		if words[i] != "" {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		}
	}
	return strings.Join(words, "")
}

// FlagName returns the kebab-case selection flag name for code, such as
// "secret-is-not-encrypted" for SECRET_IS_NOT_ENCRYPTED.
func FlagName(code Code) string {
	return strings.ReplaceAll(strings.ToLower(string(code)), "_", "-")
}

// codeFromConfigKey converts a camelCase key to its SCREAMING_SNAKE code.
func codeFromConfigKey(key string) Code {
	var b strings.Builder
	for i, r := range key {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteString(strings.ToUpper(string(r)))
	}
	return Code(b.String())
}

// Resolve converts configured overrides, keyed by camelCase code (for example
// "secretIsNotReferenced") with an off, warn, or error value, into levels keyed
// by canonical code. Off resolves to severity.None. Empty input yields a nil
// map, meaning every code keeps its default.
func Resolve(overrides map[string]string) (map[Code]severity.Level, error) {
	if len(overrides) == 0 {
		return nil, nil
	}
	out := make(map[Code]severity.Level, len(overrides))
	for key, raw := range overrides {
		if !camelCaseKey.MatchString(key) {
			return nil, &InvalidKeyError{Key: key}
		}
		code := codeFromConfigKey(key)
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

// InvalidKeyError reports an override key that is not camelCase.
type InvalidKeyError struct {
	Key string
}

func (e *InvalidKeyError) Error() string {
	return "validate check " + e.Key + " must be camelCase"
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
