package validate

import (
	"errors"
	"maps"
	"slices"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

// Service grades a workspace from its merged environments and secrets store.
type Service struct {
	// projects lists the declared projects sorted by name.
	projects []string
	// environments lists the declared environments in order.
	environments []string
	// levels overrides the built-in severity per code.
	levels map[diagnostic.Code]severity.Level
	// deps opens the inspection adapters.
	deps Dependencies
}

// New privately copies the workspace configuration without performing I/O.
func New(ws workspace.Workspace, deps Dependencies) (*Service, error) {
	if deps.OpenEnv == nil {
		return nil, errors.New("environment opener is required")
	}
	if deps.OpenSecrets == nil {
		return nil, errors.New("secrets opener is required")
	}
	if deps.OpenProviders == nil {
		return nil, errors.New("providers opener is required")
	}
	levels, err := diagnostic.Resolve(ws.ValidateSeverities)
	if err != nil {
		return nil, err
	}
	return &Service{
		projects:     slices.Sorted(maps.Keys(ws.Projects)),
		environments: slices.Clone(ws.Environments),
		levels:       levels,
		deps:         deps,
	}, nil
}
