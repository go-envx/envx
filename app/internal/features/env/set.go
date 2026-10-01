package env

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// SetParams defines input parameters for setting an environment variable in an
// overlay.
type SetParams struct {
	// IncludePath identifies the target overlay from a project's includes list.
	IncludePath string
	// Key is the dot-separated key path to write.
	Key string
	// Value is the value to write at the key path.
	Value string
	// Environment overrides the target environment; an empty value uses the
	// default.
	Environment string
}

// Set validates parameters and persists an environment variable to the
// namespace's environment overlay file through the repository.
func (s *Service) Set(params SetParams) (SetResult, error) {
	if strings.TrimSpace(params.Key) == "" {
		return SetResult{}, errors.New("key cannot be empty")
	}
	if strings.TrimSpace(params.IncludePath) == "" {
		return SetResult{}, errors.New("include path cannot be empty")
	}

	environment, err := s.normalizeEnvironment(params.Environment)
	if err != nil {
		return SetResult{}, err
	}

	targetPath := params.IncludePath
	if s.params.WorkspaceDir != "" && !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(s.params.WorkspaceDir, targetPath)
	}

	if len(s.params.Includes) > 0 {
		found := false
		cleanTarget := strings.TrimSuffix(targetPath, ".yaml")
		cleanParam := strings.TrimSuffix(params.IncludePath, ".yaml")
		for _, inc := range s.params.Includes {
			cleanInc := strings.TrimSuffix(inc, ".yaml")
			if cleanInc == cleanTarget || cleanInc == cleanParam ||
				strings.HasSuffix(cleanInc, "/"+cleanParam) {
				found = true
				targetPath = cleanInc
				break
			}
		}
		if !found {
			return SetResult{}, fmt.Errorf(
				"include path %q is not declared in any workspace project",
				params.IncludePath,
			)
		}
	}

	overlayPath, err := s.params.Repository.SetOverlay(
		targetPath, environment, params.Key, params.Value,
	)
	if err != nil {
		return SetResult{}, err
	}

	return SetResult{
		Key:         params.Key,
		OverlayPath: overlayPath,
	}, nil
}
