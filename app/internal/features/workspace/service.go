package workspace

import "errors"

// Repository abstracts workspace persistence operations consumed by Service.
type Repository interface {
	// Load discovers, reads, parses, and validates the workspace configuration.
	Load() (*Workspace, error)
}

// ServiceParams provides dependencies to the workspace domain service.
type ServiceParams struct {
	Repository Repository
}

// Service coordinates workspace loading.
type Service struct {
	params ServiceParams
}

// NewService constructs a workspace domain service.
func NewService(params ServiceParams) (*Service, error) {
	if params.Repository == nil {
		return nil, errors.New("repository is required")
	}
	return &Service{params: params}, nil
}

// Load retrieves and validates the workspace domain entity from the repository.
func (s *Service) Load() (*Workspace, error) {
	return s.params.Repository.Load()
}
