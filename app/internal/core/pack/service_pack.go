package pack

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/lib/filex"
)

// PackParams are the caller's selections and destination.
type PackParams struct {
	// Projects selects the projects to bundle; empty selects every project.
	Projects []string
	// Environments selects the overlays to bundle; empty selects every
	// declared environment.
	Environments []string
	// OutDir is the destination directory.
	OutDir string
	// Force replaces an existing non-empty destination.
	Force bool
}

// PackResult reports what a completed pack published.
type PackResult struct {
	// OutDir is the absolute destination.
	OutDir string
	// ManifestFile is the bundle-relative manifest filename.
	ManifestFile string
	// Files are the bundle-relative paths written, sorted.
	Files []string
	// Projects are the bundled projects, sorted by name.
	Projects []string
	// Environments are the bundled environments, in declared order.
	Environments []string
}

// Pack plans, stages, and publishes a bundle into params.OutDir. A non-empty
// destination is refused unless params.Force is set; a destination that is or
// contains a source is always refused. Nothing is published unless every read
// and write succeeds, and a replaced destination survives a failed swap.
func (s *Service) Pack(params PackParams) (PackResult, error) {
	if strings.TrimSpace(params.OutDir) == "" {
		return PackResult{}, ErrOutputRequired
	}
	outDir, err := filepath.Abs(params.OutDir)
	if err != nil {
		return PackResult{}, fmt.Errorf(
			"resolving output directory %q: %w", params.OutDir, err,
		)
	}
	if !params.Force {
		if err := ensureOutDirEmpty(outDir); err != nil {
			return PackResult{}, err
		}
	}

	if len(s.environments) == 0 {
		return PackResult{}, ErrNoEnvironments
	}
	environments, err := selectDeclared(
		s.environments, params.Environments,
		func(name string) string { return name }, ErrEnvironmentNotDeclared,
	)
	if err != nil {
		return PackResult{}, err
	}
	projects, err := selectDeclared(
		s.projects, params.Projects,
		func(p project) string { return p.name }, ErrProjectNotDeclared,
	)
	if err != nil {
		return PackResult{}, err
	}

	hasStore := s.secretsPath != "" && exists(s.secretsPath)
	reserved := []string{bundleManifestName, "envx"}
	if hasStore {
		reserved = append(reserved, bundleSecretsName, "secrets")
	}
	dirs, err := assignProjectDirs(projects, reserved)
	if err != nil {
		return PackResult{}, err
	}
	bundles, err := planProjects(projects, environments, dirs)
	if err != nil {
		return PackResult{}, err
	}
	if err := s.ensureOutsideSources(outDir); err != nil {
		return PackResult{}, err
	}
	var b bundle
	if b.items, err = planFiles(bundles); err != nil {
		return PackResult{}, err
	}

	manifest, err := filex.Read(s.manifestPath)
	if err != nil {
		return PackResult{}, fmt.Errorf("reading manifest %s: %w", s.manifestPath, err)
	}
	if b.manifest, err = rewriteManifest(manifest, bundles); err != nil {
		return PackResult{}, err
	}
	if hasStore {
		if b.store, err = s.exportSecrets(b.items); err != nil {
			return PackResult{}, err
		}
	}

	if err := stage(outDir, b, params.Force); err != nil {
		return PackResult{}, err
	}

	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.name
	}
	return PackResult{
		OutDir:       outDir,
		ManifestFile: bundleManifestName,
		Files:        b.files(),
		Projects:     names,
		Environments: environments,
	}, nil
}

// ensureOutsideSources rejects an existing destination that is, or is an
// ancestor of, any protected location. Comparing file identity rather than
// path strings also catches symlinks, case-insensitive names, and bind mounts.
func (s *Service) ensureOutsideSources(outDir string) error {
	dest, err := os.Stat(outDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspecting output directory %s: %w", outDir, err)
	}
	for _, source := range s.protected {
		if s.isWithin(source, dest) {
			return fmt.Errorf("%w: %s", ErrOutputContainsSource, source)
		}
	}
	return nil
}

// isWithin reports whether source or any ancestor, as written or with symlinks
// resolved, is dest.
func (s *Service) isWithin(source string, dest fs.FileInfo) bool {
	paths := []string{source}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		paths = append(paths, resolved)
	}
	for _, path := range paths {
		for {
			if info, err := os.Stat(path); err == nil && os.SameFile(dest, info) {
				return true
			}
			parent := filepath.Dir(path)
			if parent == path {
				break
			}
			path = parent
		}
	}
	return false
}
