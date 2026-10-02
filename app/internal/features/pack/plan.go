package pack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/utils/filex"
)

// planProjects discovers each selected project's namespaces and assigns their
// filenames within the project's bundle directory. Discovery is per project: a
// namespace two projects both include is discovered — and later copied — into each
// project's directory, so the bundle keeps no cross-project shared-file concept.
func planProjects(
	root string, projects []Project, environments []string, dirs map[string]string,
) ([]projectBundle, error) {
	bundles := make([]projectBundle, 0, len(projects))
	for _, project := range projects {
		includes, err := discoverProjectIncludes(root, project, environments)
		if err != nil {
			return nil, err
		}
		names, err := assignProjectNames(includes)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, projectBundle{
			name:     project.Name,
			dir:      dirs[project.Name],
			includes: includes,
			names:    names,
		})
	}
	return bundles, nil
}

// discoverProjectIncludes collects one project's namespaces and, for each, the
// base file and the existing overlay files for the selected environments. A
// namespace listed twice in the same project is discovered once. An include that
// matches no file on disk is a hard error, so a manifest typo cannot ship a
// bundle missing a namespace.
func discoverProjectIncludes(
	root string, project Project, environments []string,
) ([]includeFiles, error) {
	seen := make(map[string]struct{})
	var includes []includeFiles

	for _, include := range project.Includes {
		if _, ok := seen[include]; ok {
			continue
		}
		seen[include] = struct{}{}

		prefix := filepath.Join(root, include)
		files := includeFiles{rel: include}

		if base := prefix + ".yaml"; exists(base) {
			files.base = base
		}
		for _, env := range environments {
			overlay := prefix + "." + env + ".yaml"
			if exists(overlay) {
				files.overlays = append(files.overlays, overlayFile{env: env, src: overlay})
			}
		}

		if files.base == "" && len(files.overlays) == 0 {
			return nil, fmt.Errorf(
				"include %q in project %q matches no base file or selected overlay",
				include, project.Name,
			)
		}
		includes = append(includes, files)
	}
	return includes, nil
}

// planFiles turns the per-project bundles into the ordered copy plan: each
// namespace's base file as "<project>/<stem>.yaml" and each overlay as
// "<project>/<stem>.<env>.yaml". Destinations use "/" so the bundle-relative
// paths render and compare uniformly across platforms. The secrets store is
// handled separately because pack filters it rather than copying it verbatim.
func planFiles(bundles []projectBundle) []copyItem {
	var items []copyItem
	for _, bundle := range bundles {
		for _, include := range bundle.includes {
			stem := bundle.names[include.rel]
			if include.base != "" {
				items = append(items, copyItem{
					src:  include.base,
					dest: bundle.dir + "/" + stem + ".yaml",
				})
			}
			for _, overlay := range include.overlays {
				items = append(items, copyItem{
					src:  overlay.src,
					dest: bundle.dir + "/" + stem + "." + overlay.env + ".yaml",
				})
			}
		}
	}
	return items
}

// allIncludeFiles flattens every bundle's namespaces into one slice, for scanning
// the union of the selected projects' secret references. Duplicates from a shared
// namespace are harmless: scanReferences de-duplicates by reference.
func allIncludeFiles(bundles []projectBundle) []includeFiles {
	var all []includeFiles
	for _, bundle := range bundles {
		all = append(all, bundle.includes...)
	}
	return all
}

// selectEnvironments resolves a requested subset against the declared
// environments, preserving declared order and rejecting an unknown request. An
// empty request selects every declared environment.
func selectEnvironments(declared, requested []string) ([]string, error) {
	if len(requested) == 0 {
		if len(declared) == 0 {
			return nil, ErrNoEnvironments
		}
		return append([]string(nil), declared...), nil
	}

	declaredSet := make(map[string]struct{}, len(declared))
	for _, value := range declared {
		declaredSet[value] = struct{}{}
	}

	requestedSet := make(map[string]struct{}, len(requested))
	for _, value := range requested {
		if _, ok := declaredSet[value]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrEnvironmentNotDeclared, value)
		}
		requestedSet[value] = struct{}{}
	}

	// Walk the declared order and keep the requested ones, so the selection is
	// deterministic regardless of flag order.
	selected := make([]string, 0, len(requestedSet))
	for _, value := range declared {
		if _, ok := requestedSet[value]; ok {
			selected = append(selected, value)
		}
	}
	return selected, nil
}

// selectProjects resolves the requested project subset against the declared
// projects, preserving declared (sorted) order. An empty request selects every
// project; an unknown name is rejected.
func selectProjects(declared []Project, requested []string) ([]Project, error) {
	if len(requested) == 0 {
		return declared, nil
	}

	byName := make(map[string]Project, len(declared))
	names := make([]string, 0, len(declared))
	for _, project := range declared {
		byName[project.Name] = project
		names = append(names, project.Name)
	}

	requestedSet := make(map[string]struct{}, len(requested))
	for _, name := range requested {
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrProjectNotDeclared, name)
		}
		requestedSet[name] = struct{}{}
	}

	selected := make([]Project, 0, len(requestedSet))
	for _, name := range names {
		if _, ok := requestedSet[name]; ok {
			selected = append(selected, byName[name])
		}
	}
	return selected, nil
}

// stemOf returns a filename without its ".yaml" extension, used to reserve the
// manifest and secrets store names so no project directory collides with them.
func stemOf(name string) string {
	return strings.TrimSuffix(name, ".yaml")
}

// ensureOutDirEmpty rejects a destination that would otherwise be written into
// blindly: an existing path that is not a directory, or a directory that already
// holds entries. A missing path is fine — it is created during the write. It
// removes nothing, so the caller can fail without disturbing an existing bundle;
// replacing a non-empty directory is the --force path instead.
func ensureOutDirEmpty(outDir string) error {
	info, err := os.Stat(outDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspecting output directory %s: %w", outDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf(
			"%w: %s (use --force to replace it)", ErrOutputNotDirectory, outDir,
		)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("reading output directory %s: %w", outDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%w: %s (use --force to replace it)", ErrOutputNotEmpty, outDir)
	}
	return nil
}

// copyInto copies item.src to item.dest under outDir.
func copyInto(outDir string, item copyItem) error {
	data, err := filex.Read(item.src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", item.src, err)
	}
	return writeFile(filepath.Join(outDir, item.dest), data)
}

// writeFile writes data to target atomically, creating parent directories as
// needed. The secrets store is written separately with owner-only permissions;
// namespace and manifest files are ordinary workspace files.
func writeFile(target string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("creating bundle directory for %s: %w", target, err)
	}
	return filex.WriteAtomic(target, data)
}

// exists reports whether path names an existing regular file, treating any stat
// error (including a missing file) as absent so an optional overlay or secrets
// store is simply skipped.
func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
