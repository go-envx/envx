package flagspec

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Value constrains the supported flag value types.
type Value interface {
	string | bool | []string
}

// Spec identifies one flag and its optional environment variable fallback.
type Spec[T Value] struct {
	// Name is the long flag name, e.g. "env" for --env.
	Name string
	// Short is the one-letter shorthand, e.g. "E" for -E.
	Short string
	// Env is the environment variable fallback; empty means none.
	Env string
	// Usage describes the flag in help output.
	Usage string
	// Default is the value when neither the flag nor Env is set.
	Default T
}

// Bind registers the flag on cmd's local flags.
func (s Spec[T]) Bind(cmd *cobra.Command) {
	s.bind(cmd.Flags())
}

// BindPersistent registers the flag on cmd's persistent flags, inherited by
// its subcommands.
func (s Spec[T]) BindPersistent(cmd *cobra.Command) {
	s.bind(cmd.PersistentFlags())
}

// Value returns the flag or environment variable value, else the default.
func (s Spec[T]) Value(cmd *cobra.Command) T {
	if v := s.Lookup(cmd); v != nil {
		return *v
	}
	return s.Default
}

// Lookup returns the explicitly changed flag value, then a non-empty Env
// value, else nil. A flag not registered on cmd yields nil without consulting
// Env. Invalid values are reported on cmd's error stream and ignored.
func (s Spec[T]) Lookup(cmd *cobra.Command) *T {
	fs := cmd.Flags()
	if fs.Lookup(s.Name) == nil {
		return nil
	}
	if fs.Changed(s.Name) {
		v, err := s.changedValue(fs)
		if err != nil {
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(), "warning: invalid flag %q: %v\n", s.Name, err,
			)
			return nil
		}
		return &v
	}
	if s.Env == "" {
		return nil
	}
	raw, ok := os.LookupEnv(s.Env)
	if !ok || raw == "" {
		return nil
	}
	v, err := parseFlagEnv[T](raw)
	if err != nil {
		_, _ = fmt.Fprintf(
			cmd.ErrOrStderr(), "warning: invalid value %q for %s: %v\n", raw, s.Env, err,
		)
		return nil
	}
	return &v
}

// bind registers the flag on fs.
func (s Spec[T]) bind(fs *pflag.FlagSet) {
	switch fallback := any(s.Default).(type) {
	case string:
		fs.StringP(s.Name, s.Short, fallback, s.helpText())
	case bool:
		fs.BoolP(s.Name, s.Short, fallback, s.helpText())
	case []string:
		fs.StringSliceP(s.Name, s.Short, fallback, s.helpText())
	}
}

// helpText appends the environment variable hint to the usage, if any.
func (s Spec[T]) helpText() string {
	if s.Env == "" {
		return s.Usage
	}
	return fmt.Sprintf("%s (env: %s)", s.Usage, s.Env)
}

// changedValue reads the parsed flag value from fs.
func (s Spec[T]) changedValue(fs *pflag.FlagSet) (T, error) {
	var (
		v   any
		err error
	)
	switch any(s.Default).(type) {
	case string:
		v, err = fs.GetString(s.Name)
	case bool:
		v, err = fs.GetBool(s.Name)
	case []string:
		v, err = fs.GetStringSlice(s.Name)
	}
	if err != nil {
		return s.Default, err
	}
	return v.(T), nil
}

// parseFlagEnv converts an environment variable value into T; lists are
// comma-separated.
func parseFlagEnv[T Value](raw string) (T, error) {
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
