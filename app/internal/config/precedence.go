package config

import (
	"os"
	"strconv"

	"github.com/go-envx/envx/app/internal/schema"
)

// precedenceString resolves a string setting: the explicit value wins when present,
// then the ENVX_* var, then the first non-empty manifest layer (e.g. project then
// global default), and finally "". Nil and empty manifest layers are both skipped.
// It reads the setting's ENVX_* fallback straight from its schema.FlagSpec, so
// registration and resolution can never disagree about a name.
func precedenceString(s *schema.FlagSpec, explicit *string, layers ...*string) string {
	if explicit != nil {
		return *explicit
	}
	if s.Env != "" {
		if v, ok := os.LookupEnv(s.Env); ok {
			return v
		}
	}
	for _, layer := range layers {
		if layer != nil && *layer != "" {
			return *layer
		}
	}
	return ""
}

// precedenceBool resolves a boolean setting: the explicit value wins when present,
// then the ENVX_* var (parsed), then the first non-nil layer (e.g. project then
// global setting), and finally false.
func precedenceBool(s *schema.FlagSpec, explicit *bool, layers ...*bool) bool {
	if explicit != nil {
		return *explicit
	}
	if s.Env != "" {
		if v, ok := os.LookupEnv(s.Env); ok {
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		}
	}
	for _, layer := range layers {
		if layer != nil {
			return *layer
		}
	}
	return false
}
