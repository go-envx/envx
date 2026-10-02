package flags

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

// Value constrains the supported flag value types.
type Value interface {
	string | bool | []string
}

// Spec is one setting or CLI flag's identity parameterized by its value type.
type Spec[T Value] struct {
	// Name is the long-form flag name, e.g. "env" for "--env".
	Name string
	// Short is the short-form flag name, e.g. "E" for "-E".
	Short string
	// Env is the environment variable fallback ("" = none).
	Env string
	// Usage is the human-readable description for the flag usage string.
	Usage string
	// Default is the fallback value used when the flag is unset.
	Default T
}

// HelpText renders the usage string with the env-var hint appended when the
// setting has an environment variable fallback,
// e.g. "target environment (env: ENVX_ENV)".
func (s *Spec[T]) HelpText() string {
	if s.Env == "" {
		return s.Usage
	}
	return fmt.Sprintf("%s (env: %s)", s.Usage, s.Env)
}

// Get returns the flag's parsed value from fs, or the environment variable
// fallback if defined, or s.Default if unset or unregistered.
func (s *Spec[T]) Get(fs *pflag.FlagSet) T {
	if opt := s.GetOpt(fs); opt != nil {
		return *opt
	}
	return s.Default
}

// GetOpt returns a pointer to the flag's value if the flag was explicitly
// changed on fs, or the parsed value of its environment variable fallback
// if present and non-empty, or nil otherwise (including when the flag was
// never registered).
func (s *Spec[T]) GetOpt(fs *pflag.FlagSet) *T {
	if fs.Changed(s.Name) {
		val, err := s.getValueFromFlag(fs)
		if err != nil {
			fmt.Fprintf(
				os.Stderr, "warning: invalid flag %q: %v\n", s.Name, err,
			)
			return nil
		}
		return &val
	}
	if s.Env != "" {
		if raw, ok := os.LookupEnv(s.Env); ok && raw != "" {
			val, err := parseValueFromEnv[T](raw)
			if err != nil {
				fmt.Fprintf(
					os.Stderr,
					"warning: invalid value %q for %s: %v\n",
					raw, s.Env, err,
				)
				return nil
			}
			return &val
		}
	}
	return nil
}

// getValueFromFlag reads the parsed flag value from fs.
func (s *Spec[T]) getValueFromFlag(fs *pflag.FlagSet) (T, error) {
	var zero T
	switch any(zero).(type) {
	case string:
		v, err := fs.GetString(s.Name)
		if err != nil {
			return s.Default, err
		}
		return any(v).(T), nil
	case bool:
		v, err := fs.GetBool(s.Name)
		if err != nil {
			return s.Default, err
		}
		return any(v).(T), nil
	case []string:
		v, err := fs.GetStringSlice(s.Name)
		if err != nil {
			return s.Default, err
		}
		return any(v).(T), nil
	default:
		return s.Default, fmt.Errorf("unsupported flag type")
	}
}

// parseValueFromEnv converts an environment variable string into type T.
func parseValueFromEnv[T Value](raw string) (T, error) {
	var zero T
	switch any(zero).(type) {
	case string:
		return any(raw).(T), nil
	case bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return zero, err
		}
		return any(b).(T), nil
	case []string:
		parts := strings.Split(raw, ",")
		return any(parts).(T), nil
	default:
		return zero, fmt.Errorf("unsupported environment variable type")
	}
}
