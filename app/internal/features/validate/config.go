package validate

import (
	"errors"
	"slices"

	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// Config is everything the workspace manifest configures for validation.
type Config struct {
	// Projects lists every project to diagnose, sorted by name.
	Projects []string
	// Environments lists the declared environments every project is diagnosed
	// against.
	Environments []string
	// Severity overrides the default reporting level per status code, keyed by
	// canonical code. A nil map leaves every code at its default; a code mapped to
	// status.Off suppresses its findings entirely.
	Severity map[string]status.Severity
}

// LoadConfig derives the validation config from a loaded workspace.
func LoadConfig(ws *workspace.Workspace) (Config, error) {
	if ws == nil {
		return Config{}, errors.New("workspace is required")
	}

	severity, err := status.Resolve(ws.ValidateSeverities)
	if err != nil {
		return Config{}, err
	}

	projects := make([]string, 0, len(ws.Projects))
	for name := range ws.Projects {
		projects = append(projects, name)
	}
	slices.Sort(projects)

	return Config{
		Projects:     projects,
		Environments: slices.Clone(ws.Environments),
		Severity:     severity,
	}, nil
}
