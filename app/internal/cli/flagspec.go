package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagValue constrains the supported flag value types.
type flagValue interface {
	string | bool | []string
}

// flagSpec identifies one flag and its optional environment variable fallback.
type flagSpec[T flagValue] struct {
	// name is the long flag name, e.g. "env" for --env.
	name string
	// short is the one-letter shorthand, e.g. "E" for -E.
	short string
	// envVar is the environment variable fallback; empty means none.
	envVar string
	// usage describes the flag in help output.
	usage string
	// fallback is the value when neither the flag nor envVar is set.
	fallback T
}

// bind registers the flag on fs.
func (s *flagSpec[T]) bind(fs *pflag.FlagSet) {
	switch fallback := any(s.fallback).(type) {
	case string:
		fs.StringP(s.name, s.short, fallback, s.helpText())
	case bool:
		fs.BoolP(s.name, s.short, fallback, s.helpText())
	case []string:
		fs.StringSliceP(s.name, s.short, fallback, s.helpText())
	}
}

// value returns the flag or environment variable value, else the fallback.
func (s *flagSpec[T]) value(cmd *cobra.Command) T {
	if v := s.lookup(cmd); v != nil {
		return *v
	}
	return s.fallback
}

// lookup returns the explicitly changed flag value, then a non-empty envVar
// value, else nil. A flag not registered on cmd yields nil without consulting
// envVar. Invalid values are reported on cmd's error stream and ignored.
func (s *flagSpec[T]) lookup(cmd *cobra.Command) *T {
	fs := cmd.Flags()
	if fs.Lookup(s.name) == nil {
		return nil
	}
	if fs.Changed(s.name) {
		v, err := s.changedValue(fs)
		if err != nil {
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(), "warning: invalid flag %q: %v\n", s.name, err,
			)
			return nil
		}
		return &v
	}
	if s.envVar == "" {
		return nil
	}
	raw, ok := os.LookupEnv(s.envVar)
	if !ok || raw == "" {
		return nil
	}
	v, err := parseFlagEnv[T](raw)
	if err != nil {
		_, _ = fmt.Fprintf(
			cmd.ErrOrStderr(), "warning: invalid value %q for %s: %v\n", raw, s.envVar, err,
		)
		return nil
	}
	return &v
}

// helpText appends the environment variable hint to the usage, if any.
func (s *flagSpec[T]) helpText() string {
	if s.envVar == "" {
		return s.usage
	}
	return fmt.Sprintf("%s (env: %s)", s.usage, s.envVar)
}

// changedValue reads the parsed flag value from fs.
func (s *flagSpec[T]) changedValue(fs *pflag.FlagSet) (T, error) {
	var (
		v   any
		err error
	)
	switch any(s.fallback).(type) {
	case string:
		v, err = fs.GetString(s.name)
	case bool:
		v, err = fs.GetBool(s.name)
	case []string:
		v, err = fs.GetStringSlice(s.name)
	}
	if err != nil {
		return s.fallback, err
	}
	return v.(T), nil
}

// parseFlagEnv converts an environment variable value into T; lists are
// comma-separated.
func parseFlagEnv[T flagValue](raw string) (T, error) {
	var zero T
	var v any
	switch any(zero).(type) {
	case string:
		v = raw
	case bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return zero, err
		}
		v = b
	case []string:
		v = strings.Split(raw, ",")
	}
	return v.(T), nil
}
