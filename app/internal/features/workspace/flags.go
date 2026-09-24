package workspace

import "github.com/go-envx/envx/app/internal/utils/cliflags"

// ConfigFlag selects the manifest path (auto-discovered when unset).
var ConfigFlag = cliflags.FlagSpec{
	Name:  "config",
	Env:   "ENVX_CONFIG",
	Usage: "path to envx.yaml, or a directory containing it",
}
