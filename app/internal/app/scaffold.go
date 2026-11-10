package app

import (
	"github.com/go-envx/envx/app/internal/core/scaffold"
)

// NewScaffoldService returns the scaffold service over the embedded templates; it
// needs no workspace.
func (Factory) NewScaffoldService() (*scaffold.Service, error) {
	return &scaffold.Service{}, nil
}
