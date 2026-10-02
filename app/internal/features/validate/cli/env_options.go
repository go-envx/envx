package cli

import (
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/spf13/pflag"
)

// bindEnvOptionsFlags registers the shared environment-setting flags on fs. The
// environment itself is not among them: validate diagnoses every declared
// environment rather than selecting one.
func bindEnvOptionsFlags(fs *pflag.FlagSet) {
	flags.Bind(fs, &delimiterFlag)
	flags.Bind(fs, &overloadFlag)
	flags.Bind(fs, &prefixFlag)
	flags.Bind(fs, &referencePatternFlag)
	flags.Bind(fs, &requireOverlaysFlag)
	flags.Bind(fs, &suffixFlag)
}

// getEnvOptions returns the setting options for the flags explicitly changed on
// fs. A setting is present only when the user changed its flag, so an unchanged
// flag resolves to nil and falls through to the ENVX_* var and manifest layers.
func getEnvOptions(fs *pflag.FlagSet) env.Options {
	return env.Options{
		Delimiter:        delimiterFlag.GetOpt(fs),
		Overload:         overloadFlag.GetOpt(fs),
		Prefix:           prefixFlag.GetOpt(fs),
		ReferencePattern: referencePatternFlag.GetOpt(fs),
		RequireOverlays:  requireOverlaysFlag.GetOpt(fs),
		Suffix:           suffixFlag.GetOpt(fs),
	}
}
