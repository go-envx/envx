package pack

import "errors"

var (
	// ErrOutputRequired indicates that no output directory was requested.
	ErrOutputRequired = errors.New("output directory is required")
	// ErrOutputNotEmpty indicates that the output directory exists and holds entries.
	ErrOutputNotEmpty = errors.New("output directory is not empty")
	// ErrOutputNotDirectory indicates that the output path exists but is not a directory.
	ErrOutputNotDirectory = errors.New("output path exists and is not a directory")
	// ErrNoEnvironments indicates that the workspace declares no environments.
	ErrNoEnvironments = errors.New("no environments are declared in the manifest")
	// ErrEnvironmentNotDeclared indicates that an unknown environment was requested.
	ErrEnvironmentNotDeclared = errors.New("environment is not declared in the manifest")
	// ErrProjectNotDeclared indicates that an unknown project was requested.
	ErrProjectNotDeclared = errors.New("project is not declared in the manifest")
)
