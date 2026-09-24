package flags

import (
	"github.com/spf13/pflag"
)

// Option registers one setting's flag on a flag set. Each action lists the options
// it wants explicitly, so a command's flag surface is visible at its registration
// site rather than hidden behind a bundle.
type Option func(*pflag.FlagSet)

// Register applies each option to fs. A command chooses the scope by which flag set
// it passes: cmd.Flags() for local flags, cmd.PersistentFlags() for the inherited
// --config bootstrap flag.
func Register(fs *pflag.FlagSet, opts ...Option) {
	for _, opt := range opts {
		opt(fs)
	}
}

// WithConfig registers the --config flag selecting the manifest. Root registers it
// on its persistent flag set so every subcommand inherits it.
func WithConfig(fs *pflag.FlagSet) {
	registerString(fs, &Config)
}

// WithEnv registers the --env flag, the target environment to resolve.
func WithEnv(fs *pflag.FlagSet) {
	registerString(fs, &Env)
}

// WithRequireOverlays registers the --require-overlays flag, requiring every
// overlay file to exist.
func WithRequireOverlays(fs *pflag.FlagSet) {
	registerBool(fs, &RequireOverlays)
}

// WithPrefix registers the --prefix flag, prepended to every resolved key.
func WithPrefix(fs *pflag.FlagSet) {
	registerString(fs, &Prefix)
}

// WithSuffix registers the --suffix flag, appended to every resolved key.
func WithSuffix(fs *pflag.FlagSet) {
	registerString(fs, &Suffix)
}

// WithDelimiter registers the --delimiter flag, the string used to join a
// list-valued setting into a single env var.
func WithDelimiter(fs *pflag.FlagSet) {
	registerString(fs, &Delimiter)
}

// WithOverload registers the --overload flag, letting file values win over OS env
// vars; only run hands the merged environment to the runner.
func WithOverload(fs *pflag.FlagSet) {
	registerBool(fs, &Overload)
}

// WithReferencePattern registers the --reference-pattern flag, overriding the
// {{VAR}} reference syntax with a custom regex.
func WithReferencePattern(fs *pflag.FlagSet) {
	registerString(fs, &ReferencePattern)
}

// registerString registers spec as a dest-less string flag on fs, delegating to
// BindString and discarding the destination since GetInput reads the value back
// from the flag set.
func registerString(fs *pflag.FlagSet, spec *FlagSpec) {
	BindString(fs, new(string), spec)
}

// registerBool registers spec as a dest-less bool flag on fs, the boolean
// counterpart to registerString.
func registerBool(fs *pflag.FlagSet, spec *FlagSpec) {
	BindBool(fs, new(bool), spec)
}
