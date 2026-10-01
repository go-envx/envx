// Package cli provides CLI commands for environment resolution,
// inspection, and modification.
package cli

import (
	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
)

// Factory defines the capabilities required for the environment commands.
type Factory interface {
	EnvService(in *core.Input, project string) (*env.Service, error)
}
