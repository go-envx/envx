package core

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/spf13/pflag"
)

// GetInput extracts the explicitly-set setting flags from fs into a *Input
// for resolution, including the --config flag. A setting is present only when the
// user changed its flag; a flag never registered is never changed, so it resolves
// to nil and falls through to the ENVX_* var and manifest layers.
func GetInput(fs *pflag.FlagSet) *Input {
	return &Input{
		ConfigPath:       optString(fs, &flags.Config),
		Env:              optString(fs, &flags.Env),
		RequireOverlays:  optBool(fs, &flags.RequireOverlays),
		Prefix:           optString(fs, &flags.Prefix),
		Suffix:           optString(fs, &flags.Suffix),
		Delimiter:        optString(fs, &flags.Delimiter),
		Overload:         optBool(fs, &flags.Overload),
		ReferencePattern: optString(fs, &flags.ReferencePattern),
	}
}

// optString returns a pointer to the flag's value when the user explicitly set it,
// and nil otherwise (including when the flag was never registered).
func optString(fs *pflag.FlagSet, s *flags.FlagSpec) *string {
	if !fs.Changed(s.Name) {
		return nil
	}
	v, _ := fs.GetString(s.Name)
	return &v
}

// optBool returns a pointer to the flag's value when the user explicitly set it,
// and nil otherwise (including when the flag was never registered).
func optBool(fs *pflag.FlagSet, s *flags.FlagSpec) *bool {
	if !fs.Changed(s.Name) {
		return nil
	}
	v, _ := fs.GetBool(s.Name)
	return &v
}
