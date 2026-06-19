package config

import (
	"os"
	"strconv"

	"github.com/go-envx/envx/apps/envx/internal/flags"
)

// -------------------------------------------------------------------------------------
// EnvLookup is the signature for environment-variable lookup. It defaults to
// os.LookupEnv but can be replaced in tests.
type EnvLookup func(key string) (string, bool)

// -------------------------------------------------------------------------------------
// FlagSet is the slice of cobra's flag state the resolver depends on. Accepting
// this interface (rather than *pflag.FlagSet) keeps config free of any CLI
// framework.
type FlagSet interface {
	Changed(name string) bool
}

// -------------------------------------------------------------------------------------
// Resolve assembles the shared Global root context by discovering and loading
// the manifest. Environment selection is no longer part of the root context:
// each action resolves its own target environment as a per-action setting.
func Resolve(configFlag string) (Global, error) {
	path, err := Discover(configFlag)
	if err != nil {
		return Global{}, err
	}
	cfg, err := Load(path)
	if err != nil {
		return Global{}, err
	}

	return Global{
		Config:     cfg,
		ConfigPath: path,
	}, nil
}

// -------------------------------------------------------------------------------------
// Resolver applies the precedence "explicit flag > ENVX_* env var > layered
// defaults". It reads each flag's name and ENVX_* fallback straight from its
// flags.Spec, so registration and resolution can never disagree about a name.
type Resolver struct {
	LookupEnv EnvLookup
}

// -------------------------------------------------------------------------------------
// NewResolver creates a Resolver backed by os.LookupEnv.
func NewResolver() *Resolver {
	return &Resolver{LookupEnv: os.LookupEnv}
}

// -------------------------------------------------------------------------------------
// String resolves a string setting: the flag value wins when the user set it,
// then the ENVX_* var, then the first non-empty layer (e.g. project then global
// default), and finally "".
func (r *Resolver) String(
	s flags.Spec, changed FlagSet, flagVal string, layers ...string,
) string {
	if changed != nil && changed.Changed(s.Name) {
		return flagVal
	}
	if s.Env != "" {
		if v, ok := r.LookupEnv(s.Env); ok {
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
// Bool resolves a boolean setting: the flag value wins when the user set it,
// then the ENVX_* var (parsed), then the first non-nil layer (e.g. project then
// global setting), and finally false.
func (r *Resolver) Bool(
	s flags.Spec, changed FlagSet, flagVal bool, layers ...*bool,
) bool {
	if changed != nil && changed.Changed(s.Name) {
		return flagVal
	}
	if s.Env != "" {
		if v, ok := r.LookupEnv(s.Env); ok {
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
