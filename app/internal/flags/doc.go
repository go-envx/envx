// Package flags translates schema flag specs into pflag flags and reads them back.
//
// Resolution flags flow through config: an action registers them onto a flag set
// with Register(fs, WithEnv, WithPrefix, ...) and GetInput(fs) reads them back into
// a config.Input, sourcing values straight from the flag set so registration and
// extraction cannot drift. A command chooses the scope by which flag set it passes
// — cmd.Flags() for local flags, root.PersistentFlags() for the inherited --config
// bootstrap flag. Presentation flags a command consumes directly (--output)
// register into a local variable via BindString.
package flags
