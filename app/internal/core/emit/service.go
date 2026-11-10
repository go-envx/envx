package emit

import "errors"

// Service renders project environments to delivery targets.
type Service struct {
	// deps holds the adapters.
	deps Dependencies
}

// New verifies every dependency is present.
func New(deps Dependencies) (*Service, error) {
	if deps.Env == nil {
		return nil, errors.New("explainer is required")
	}
	if deps.Renderer == nil {
		return nil, errors.New("renderer is required")
	}
	return &Service{deps: deps}, nil
}
