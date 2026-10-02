package core

import (
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/spf13/pflag"
)

// GetInput extracts the explicitly-set setting flags from fs into a *Input
// for resolution, including the --config flag. A setting is present only when the
// user changed its flag; a flag never registered is never changed, so it resolves
// to nil and falls through to the ENVX_* var and manifest layers.
// Retained for backward compatibility with packages pending refactoring.
func GetInput(fs *pflag.FlagSet) *Input {
	return &Input{
		ConfigPath:       flags.Config.GetOpt(fs),
		Env:              env.Env.GetOpt(fs),
		RequireOverlays:  env.RequireOverlays.GetOpt(fs),
		Prefix:           env.Prefix.GetOpt(fs),
		Suffix:           env.Suffix.GetOpt(fs),
		Delimiter:        env.Delimiter.GetOpt(fs),
		Overload:         env.Overload.GetOpt(fs),
		ReferencePattern: env.ReferencePattern.GetOpt(fs),
	}
}
