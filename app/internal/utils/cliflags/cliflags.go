// Package cliflags provides domain-blind wrappers and specifications for pflag
// flag binding.
package cliflags

import (
	"fmt"

	"github.com/spf13/pflag"
)

// FlagSpec is one setting's CLI identity.
type FlagSpec struct {
	// Name is the long-form flag name, e.g. "env" for "--env".
	Name string
	// Short is the short-form flag name, e.g. "E" for "-E".
	Short string
	// Env is the environment variable fallback ("" = none).
	Env string
	// Usage is the human-readable description for the flag usage string.
	Usage string
	// DefaultString is a string flag's fallback value, used when the flag is unset
	// (the zero value "" when not specified).
	DefaultString string
	// DefaultBool is a bool flag's fallback value, used when the flag is unset (the
	// zero value false when not specified).
	DefaultBool bool
}

// HelpText renders the usage string with the env-var hint appended when the
// setting has an environment variable fallback,
// e.g. "target environment (env: ENVX_ENV)".
func (s *FlagSpec) HelpText() string {
	if s.Env == "" {
		return s.Usage
	}
	return fmt.Sprintf("%s (env: %s)", s.Usage, s.Env)
}

// BindString registers spec as a string flag on fs, writing the parsed value into dst.
func BindString(fs *pflag.FlagSet, dst *string, spec *FlagSpec) {
	fs.StringVarP(dst, spec.Name, spec.Short, spec.DefaultString, spec.HelpText())
}

// BindBool registers spec as a bool flag on fs, writing the parsed value into dst.
func BindBool(fs *pflag.FlagSet, dst *bool, spec *FlagSpec) {
	fs.BoolVarP(dst, spec.Name, spec.Short, spec.DefaultBool, spec.HelpText())
}

// BindStringSlice registers spec as a repeatable string flag on fs, accumulating
// each occurrence into dst.
func BindStringSlice(fs *pflag.FlagSet, dst *[]string, spec *FlagSpec) {
	fs.StringSliceVarP(dst, spec.Name, spec.Short, nil, spec.HelpText())
}
