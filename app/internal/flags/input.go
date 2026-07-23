package flags

import (
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/spf13/pflag"
)

// -------------------------------------------------------------------------------------

// GetInput extracts the explicitly-set setting flags from fs into a *config.Input
// for resolution, including the --config flag. A setting is present only when the
// user changed its flag; a flag never registered is never changed, so it resolves
// to nil and falls through to the ENVX_* var and manifest layers.
func GetInput(fs *pflag.FlagSet) *config.Input {
	return &config.Input{
		ConfigPath:      optString(fs, &schema.Config),
		Env:             optString(fs, &schema.Env),
		RequireOverlays: optBool(fs, &schema.RequireOverlays),
		Prefix:          optString(fs, &schema.Prefix),
		Suffix:          optString(fs, &schema.Suffix),
		Delimiter:       optString(fs, &schema.Delimiter),
		NamespacePrefix: optBool(fs, &schema.NamespacePrefix),
		Overload:        optBool(fs, &schema.Overload),
	}
}

// -------------------------------------------------------------------------------------

// optString returns a pointer to the flag's value when the user explicitly set it,
// and nil otherwise (including when the flag was never registered).
func optString(fs *pflag.FlagSet, s *schema.FlagSpec) *string {
	if !fs.Changed(s.Name) {
		return nil
	}
	v, _ := fs.GetString(s.Name)
	return &v
}

// -------------------------------------------------------------------------------------

// optBool returns a pointer to the flag's value when the user explicitly set it,
// and nil otherwise (including when the flag was never registered).
func optBool(fs *pflag.FlagSet, s *schema.FlagSpec) *bool {
	if !fs.Changed(s.Name) {
		return nil
	}
	v, _ := fs.GetBool(s.Name)
	return &v
}
