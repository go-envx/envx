package app

import (
	"github.com/go-envx/envx/app/internal/core/emit"
	"github.com/go-envx/envx/app/internal/core/emit/format"
)

// NewEmitService assembles the emit service over the environment service for
// the workspace at configPath.
func (f Factory) NewEmitService(configPath string) (*emit.Service, error) {
	envService, err := f.NewEnvService(configPath)
	if err != nil {
		return nil, err
	}
	return emit.New(emit.Dependencies{
		Env:      envService,
		Renderer: format.Renderer{},
	})
}
