package flags

import "github.com/spf13/pflag"

// Bind registers spec as a flag on fs using spec.Default as the flag fallback.
func Bind[T Value](fs *pflag.FlagSet, spec *Spec[T]) {
	switch any(spec.Default).(type) {
	case string:
		def := any(spec.Default).(string)
		fs.StringP(spec.Name, spec.Short, def, spec.HelpText())
	case bool:
		def := any(spec.Default).(bool)
		fs.BoolP(spec.Name, spec.Short, def, spec.HelpText())
	case []string:
		def := any(spec.Default).([]string)
		fs.StringSliceP(spec.Name, spec.Short, def, spec.HelpText())
	}
}
