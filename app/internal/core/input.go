package core

import (
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/spf13/pflag"
)

// GetInput extracts the explicitly-set setting flags from fs into a *Input
// for resolution, including the --config flag. A setting is present only when the
// user changed its flag; a flag never registered is never changed, so it resolves
// to nil and falls through to the ENVX_* var and manifest layers.
func GetInput(fs *pflag.FlagSet) *Input {
	return &Input{
		ConfigPath:       optString(fs, &workspace.ConfigFlag),
		Env:              optString(fs, &env.Env),
		RequireOverlays:  optBool(fs, &env.RequireOverlays),
		Prefix:           optString(fs, &env.Prefix),
		Suffix:           optString(fs, &env.Suffix),
		Delimiter:        optString(fs, &env.Delimiter),
		Overload:         optBool(fs, &env.Overload),
		ReferencePattern: optString(fs, &env.ReferencePattern),
	}
}

// optString returns a pointer to the flag's value when the user explicitly set it,
// and nil otherwise (including when the flag was never registered).
func optString(fs *pflag.FlagSet, s *flags.Spec[string]) *string {
	if !fs.Changed(s.Name) {
		return nil
	}
	v, _ := fs.GetString(s.Name)
	return &v
}

// optBool returns a pointer to the flag's value when the user explicitly set it,
// and nil otherwise (including when the flag was never registered).
func optBool(fs *pflag.FlagSet, s *flags.Spec[bool]) *bool {
	if !fs.Changed(s.Name) {
		return nil
	}
	v, _ := fs.GetBool(s.Name)
	return &v
}
