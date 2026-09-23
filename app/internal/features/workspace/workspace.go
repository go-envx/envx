package workspace

// ProjectRef represents a declared project's includes within a workspace.
type ProjectRef struct {
	// Name is the project name declared in the manifest.
	Name string
	// Includes lists the project's ordered namespace relative paths.
	Includes []string
}

// Layout represents the file-level view of a workspace: manifest path, root
// directory, secrets and keys locations, declared environments, and projects.
type Layout struct {
	// ManifestPath is the absolute path to the manifest file.
	ManifestPath string
	// Root is the absolute root directory of the workspace.
	Root string
	// SecretsPath is the absolute path to the secrets store file.
	SecretsPath string
	// KeysPath is the absolute path to the private-key file.
	KeysPath string
	// Environments is the list of declared environments.
	Environments []string
	// Projects is the list of project includes in sorted order.
	Projects []ProjectRef
}
