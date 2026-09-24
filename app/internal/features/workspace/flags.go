package workspace

import "github.com/go-envx/envx/app/internal/shared/flags"

// ConfigFlag selects the manifest path (auto-discovered when unset).
var ConfigFlag = flags.Spec{
	Name:  "config",
	Env:   "ENVX_CONFIG",
	Usage: "path to envx.yaml, or a directory containing it",
}
