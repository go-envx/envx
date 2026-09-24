// Package flags provides flag specifications, common presentation flags,
// and binding helpers for pflag flag sets.
package flags

import (
	"fmt"

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

// Bind registers spec as a flag on fs, writing the parsed value into dst.
func Bind[T Value](fs *pflag.FlagSet, dst *T, spec *Spec[T]) {
	switch d := any(dst).(type) {
	case *string:
		def := any(spec.Default).(string)
		fs.StringVarP(d, spec.Name, spec.Short, def, spec.HelpText())
	case *bool:
		def := any(spec.Default).(bool)
		fs.BoolVarP(d, spec.Name, spec.Short, def, spec.HelpText())
	case *[]string:
		def := any(spec.Default).([]string)
		fs.StringSliceVarP(d, spec.Name, spec.Short, def, spec.HelpText())
	}
}

// Shared presentation flags.
var (
	// Output selects the rendering format for tabular commands.
	Output = Spec[string]{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}

	// Verbose prints additional detail in command output.
	Verbose = Spec[bool]{
		Name:  "verbose",
		Short: "v",
		Usage: "print detailed output",
	}
)
