package cli

import (
	"slices"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/spf13/pflag"
)

// without names a flag to omit from bindEnvFlags.
func without[T flags.Value](spec *flags.Spec[T]) string {
	return spec.Name
}

// bindEnvOptionsFlags registers the shared environment-setting flags on fs,
// except those whose names are listed in skip.
func bindEnvOptionsFlags(fs *pflag.FlagSet, skip ...string) {
	all := pflag.NewFlagSet("env", pflag.ContinueOnError)
	flags.Bind(all, &delimiterFlag)
	flags.Bind(all, &envFlag)
	flags.Bind(all, &overloadFlag)
	flags.Bind(all, &prefixFlag)
	flags.Bind(all, &referencePatternFlag)
	flags.Bind(all, &requireOverlaysFlag)
	flags.Bind(all, &suffixFlag)

	all.VisitAll(func(f *pflag.Flag) {
		if !slices.Contains(skip, f.Name) {
			fs.AddFlag(f)
		}
	})
}

// getEnvOptions returns the setting options for the flags explicitly changed on
// fs. A setting is present only when the user changed its flag; a flag never
// registered is never changed, so it resolves to nil and falls through to the
// ENVX_* var and manifest layers.
func getEnvOptions(fs *pflag.FlagSet) env.Options {
	return env.Options{
		Delimiter:        delimiterFlag.GetOpt(fs),
		Env:              envFlag.GetOpt(fs),
		Overload:         overloadFlag.GetOpt(fs),
		Prefix:           prefixFlag.GetOpt(fs),
		ReferencePattern: referencePatternFlag.GetOpt(fs),
		RequireOverlays:  requireOverlaysFlag.GetOpt(fs),
		Suffix:           suffixFlag.GetOpt(fs),
	}
}
