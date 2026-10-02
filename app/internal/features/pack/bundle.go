package pack

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
