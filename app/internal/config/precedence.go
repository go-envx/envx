package config

import (
	"os"
	"strconv"

	"github.com/go-envx/envx/app/internal/schema"
)

// -------------------------------------------------------------------------------------

// precedence applies the chain "explicit value > ENVX_* env var > layered
// defaults". It reads each setting's ENVX_* fallback straight from its
// schema.FlagSpec, so registration and resolution can never disagree about a
// name.
type precedence struct{}

// -------------------------------------------------------------------------------------

// newPrecedence creates a precedence.
func newPrecedence() *precedence {
	return &precedence{}
}

// -------------------------------------------------------------------------------------

// String resolves a string setting: the explicit value wins when present, then
// the ENVX_* var, then the first non-empty layer (e.g. project then global
// default), and finally "".
func (p *precedence) String(
	s *schema.FlagSpec, explicit *string, layers ...string,
) string {
	if explicit != nil {
		return *explicit
	}
	if s.Env != "" {
		if v, ok := os.LookupEnv(s.Env); ok {
			return v
		}
	}
	for _, layer := range layers {
		if layer != "" {
			return layer
		}
	}
	return ""
}

// -------------------------------------------------------------------------------------

// Bool resolves a boolean setting: the explicit value wins when present, then the
// ENVX_* var (parsed), then the first non-nil layer (e.g. project then global
// setting), and finally false.
func (p *precedence) Bool(
	s *schema.FlagSpec, explicit *bool, layers ...*bool,
) bool {
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
