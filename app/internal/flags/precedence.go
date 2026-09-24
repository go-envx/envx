package flags

import (
	"os"
	"strconv"
)

// PrecedenceString resolves a string setting: the explicit value wins when present,
// then the ENVX_* var, then the first non-empty layer (e.g. project then
// global default), and finally "". Nil and empty layers are both skipped.
// It reads the setting's ENVX_* fallback straight from its FlagSpec, so
// registration and resolution can never disagree about a name.
func PrecedenceString(s *FlagSpec, explicit *string, layers ...*string) string {
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

// PrecedenceBool resolves a boolean setting: the explicit value wins when present,
// then the ENVX_* var (parsed), then the first non-nil layer (e.g. project then
// global setting), and finally false.
func PrecedenceBool(s *FlagSpec, explicit *bool, layers ...*bool) bool {
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
