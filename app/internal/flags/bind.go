package flags

import (
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/spf13/pflag"
)

// -------------------------------------------------------------------------------------

// BindString registers spec as a string flag on fs, writing the parsed value into
// dst. Every string flag registration flows through here, so a spec's identity
// (name, shorthand, default, help) is applied in exactly one place. A command
// chooses the scope by which flag set it passes: cmd.Flags() for a local flag,
// cmd.PersistentFlags() for one inherited by subcommands.
func BindString(fs *pflag.FlagSet, dst *string, spec *schema.FlagSpec) {
	fs.StringVarP(dst, spec.Name, spec.Short, spec.DefaultString, spec.HelpText())
}

// -------------------------------------------------------------------------------------

// BindBool is BindString's boolean counterpart.
func BindBool(fs *pflag.FlagSet, dst *bool, spec *schema.FlagSpec) {
	fs.BoolVarP(dst, spec.Name, spec.Short, spec.DefaultBool, spec.HelpText())
}
