package pack

// Workspace is the file-level view of a resolved workspace pack copies from: the
// manifest and secrets locations, the declared environments, and each project's
// includes. It carries no resolved values because pack never decrypts — it only
// selects and copies files.
type Workspace struct {
	// ManifestPath is the absolute path of the source manifest.
	ManifestPath string
	// Root is the absolute workspace directory every relative include resolves
	// against.
	Root string
	// SecretsPath is the absolute path of the encrypted secrets store, copied when
	// it exists and skipped for a workspace that has none.
	SecretsPath string
	// Environments is the manifest's declared environment list, the default
	// selection and the set a requested environment is validated against.
	Environments []string
	// Projects is every declared project's includes.
	Projects []Project
}

// Project is one project's contribution to the bundle: its name and the ordered
// include prefixes declared in the manifest (each resolving to a base
// <prefix>.yaml and per-environment <prefix>.<env>.yaml overlays).
type Project struct {
	// Name is the manifest project name.
	Name string
	// Includes lists the project's ordered namespace prefixes, relative to Root.
	Includes []string
}

// PackParams are the caller's selections: which environments and projects to
// include and where to write the bundle.
type PackParams struct {
	// Environments selects the environments whose overlays are copied; empty means
	// every declared environment.
	Environments []string
	// Projects selects the projects whose includes are copied; empty means every
	// declared project.
	Projects []string
	// OutDir is the destination directory the bundle is written into.
	OutDir string
	// Force replaces an existing non-empty output directory: without it a
	// non-empty OutDir is refused, so a bundle never merges into stale files;
	// with it the directory is cleared just before the bundle is written.
	Force bool
}

// PackResult reports what a completed pack wrote, for rendering.
type PackResult struct {
	// OutDir is the absolute destination the bundle was written into.
	OutDir string
	// ManifestFile is the bundle-relative manifest filename, for the run hint.
	ManifestFile string
	// Files are the bundle-relative paths written, sorted for stable output.
	Files []string
	// Environments are the environments included, in declared order.
	Environments []string
	// Projects are the projects included, sorted by name.
	Projects []string
}

// copyItem pairs one source file with its bundle-relative destination.
type copyItem struct {
	// src is the absolute source path.
	src string
	// dest is the bundle-relative destination path.
	dest string
}
