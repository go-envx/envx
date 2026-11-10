package env

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// SetParams writes one value to a namespace's environment overlay.
type SetParams struct {
	// IncludePath is a declared include path, relative to the workspace root.
	IncludePath string
	// Key is the dotted key path to write.
	Key string
	// Value is the text to write.
	Value string
	// Environment overrides the target environment; nil or empty uses the
	// workspace default.
	Environment *string
}

// SetResult reports the written key and overlay file.
type SetResult struct {
	// Key is the dotted key path that was written.
	Key string
	// OverlayPath is the overlay file that received the value.
	OverlayPath string
}

// Set writes a value to the overlay of a declared include.
func (s *Service) Set(params SetParams) (SetResult, error) {
	if s.deps.EnvOverlays == nil {
		return SetResult{}, errors.New("overlay writer is required")
	}
	if strings.TrimSpace(params.Key) == "" {
		return SetResult{}, errors.New("key cannot be empty")
	}
	if strings.TrimSpace(params.IncludePath) == "" {
		return SetResult{}, errors.New("include path cannot be empty")
	}
	environment, err := s.environment(params.Environment)
	if err != nil {
		return SetResult{}, err
	}
	include, err := s.declaredInclude(params.IncludePath)
	if err != nil {
		return SetResult{}, err
	}
	overlayPath, err := s.deps.EnvOverlays.SetOverlay(
		include, environment, params.Key, params.Value,
	)
	if err != nil {
		return SetResult{}, err
	}
	return SetResult{Key: params.Key, OverlayPath: overlayPath}, nil
}

// declaredInclude returns the absolute include path a project declares for
// include. An exact root-relative or absolute match wins; otherwise a unique
// path-suffix match is required.
func (s *Service) declaredInclude(include string) (string, error) {
	target := include
	if s.root != "" && !filepath.IsAbs(target) {
		target = filepath.Join(s.root, target)
	}
	target = strings.TrimSuffix(target, ".yaml")
	param := filepath.FromSlash(strings.TrimSuffix(include, ".yaml"))
	suffix := string(filepath.Separator) + param

	var matches []string
	for _, name := range slices.Sorted(maps.Keys(s.projects)) {
		for _, path := range s.projects[name].IncludePaths {
			candidate := strings.TrimSuffix(path, ".yaml")
			if candidate == target || candidate == param {
				return candidate, nil
			}
			if strings.HasSuffix(candidate, suffix) && !slices.Contains(matches, candidate) {
				matches = append(matches, candidate)
			}
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf(
			"include path %q is not declared in any workspace project", include,
		)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf(
			"include path %q is ambiguous (matches: %s)",
			include, strings.Join(matches, ", "),
		)
	}
}
