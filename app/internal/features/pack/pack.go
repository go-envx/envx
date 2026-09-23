package pack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/utils/file"
)

// defaultIndent is the block indentation applied to a rewritten manifest whose
// source document has no detectable indentation of its own.
const defaultIndent = 2

// The bundle standardizes on envx's default filenames regardless of what the
// source workspace called its manifest and store, so every packed workspace is
// consumed by an identical command — `envx run --config <dir>/envx.yaml`. run
// auto-discovers envx.yaml and secrets.yaml is the default store location, so the
// bundle carries no custom paths (pack drops an explicit secrets path entirely).
const (
	bundleManifestName = "envx.yaml"
	bundleSecretsName  = "secrets.yaml"
)

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

// Params are the caller's selections: which environments and projects to include
// and where to write the bundle.
type Params struct {
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

// Result reports what a completed pack wrote, for rendering.
type Result struct {
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

// Pack selects the environment-scoped file set from ws and writes a per-project
// bundle into p.OutDir: one manifest at the bundle root, each selected project's
// namespace files (base and selected-env overlays) under its own <project>/
// directory, and a single filtered secrets store at the root. It rewrites the
// manifest so its includes point at the per-project paths and its secrets path is
// dropped, excludes the private-key file, and decrypts nothing. It returns the
// written file set or the first error, having written no output on a selection
// error. An existing non-empty output directory is refused unless p.Force is set,
// in which case it is cleared just before the bundle is written.
func Pack(ws Workspace, p Params) (Result, error) {
	if strings.TrimSpace(p.OutDir) == "" {
		return Result{}, errors.New("output directory is required")
	}

	outDir, err := filepath.Abs(p.OutDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolving output directory %q: %w", p.OutDir, err)
	}

	// Refuse an existing non-empty destination up front so a bundle never merges
	// into stale files; --force opts into replacing it. The directory itself is
	// not touched until just before the first write, so this check writes nothing.
	if !p.Force {
		if err := ensureOutDirEmpty(outDir); err != nil {
			return Result{}, err
		}
	}

	// Resolve the environment and project selections against the declared sets so
	// an unknown name fails before anything is written.
	environments, err := selectValues("environment", ws.Environments, p.Environments)
	if err != nil {
		return Result{}, err
	}
	projects, err := selectProjects(ws.Projects, p.Projects)
	if err != nil {
		return Result{}, err
	}

	// The bundle always uses envx's default filenames, whatever the source
	// workspace called them. Reserve those names (and their stems) so no project
	// directory collides with a root bundle file.
	manifestName := bundleManifestName
	secretsName := ""
	if ws.SecretsPath != "" && exists(ws.SecretsPath) {
		secretsName = bundleSecretsName
	}
	reserved := []string{manifestName, stemOf(manifestName)}
	if secretsName != "" {
		reserved = append(reserved, secretsName, stemOf(secretsName))
	}
	dirs, err := assignProjectDirs(projects, reserved)
	if err != nil {
		return Result{}, err
	}

	// Discover each project's namespaces and the source files each contributes for
	// the selected environments, and assign filenames within the project directory.
	bundles, err := planProjects(ws.Root, projects, environments, dirs)
	if err != nil {
		return Result{}, err
	}

	// Build the per-project copy plan for the namespace files.
	items := planFiles(bundles)

	// Rewrite the manifest so each project's includes resolve against its bundle
	// directory and any explicit secrets path is dropped.
	manifestData, err := file.Read(ws.ManifestPath)
	if err != nil {
		return Result{}, fmt.Errorf("reading manifest %s: %w", ws.ManifestPath, err)
	}
	rewritten, err := rewriteManifest(manifestData, bundles)
	if err != nil {
		return Result{}, err
	}

	// Clear a forced destination only now that every fallible pre-write step has
	// succeeded, so a --force run that fails earlier leaves any existing bundle
	// untouched. RemoveAll is a no-op when the directory is absent or empty.
	if p.Force {
		if err := os.RemoveAll(outDir); err != nil {
			return Result{}, fmt.Errorf("clearing output directory %s: %w", outDir, err)
		}
	}

	// Write the bundle: the rewritten manifest first, then every planned file.
	manifestTarget := filepath.Join(outDir, manifestName)
	if err := writeFile(manifestTarget, rewritten); err != nil {
		return Result{}, err
	}
	written := []string{manifestName}
	for _, item := range items {
		if err := copyInto(outDir, item); err != nil {
			return Result{}, err
		}
		written = append(written, item.dest)
	}

	// Copy only the secrets the selected projects' namespaces reference, dropping
	// every public key so the bundle is run-only. The store stays a single shared
	// file at the bundle root, filtered to the union of the selected projects'
	// references; a workspace with no referenced secret carries no store at all.
	if secretsName != "" {
		referenced, err := scanReferences(allIncludeFiles(bundles))
		if err != nil {
			return Result{}, err
		}
		if len(referenced) > 0 {
			dst := filepath.Join(outDir, secretsName)
			if err := secrets.WriteFilteredStore(ws.SecretsPath, dst, referenced); err != nil {
				return Result{}, err
			}
			written = append(written, secretsName)
		}
	}
	sort.Strings(written)

	projectNames := make([]string, len(projects))
	for i, project := range projects {
		projectNames[i] = project.Name
	}

	return Result{
		OutDir:       outDir,
		ManifestFile: manifestName,
		Files:        written,
		Environments: environments,
		Projects:     projectNames,
	}, nil
}

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

// selectValues resolves a requested subset against the declared set, preserving
// declared order and rejecting an unknown request. An empty request selects every
// declared value. It backs the environment selection.
func selectValues(kind string, declared, requested []string) ([]string, error) {
	if len(requested) == 0 {
		if len(declared) == 0 {
			return nil, fmt.Errorf("no %ss are declared in the manifest", kind)
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
			return nil, fmt.Errorf("%s %q is not declared in the manifest", kind, value)
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
			return nil, fmt.Errorf("project %q is not declared in the manifest", name)
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
			"output path %s already exists and is not a directory (use --force to replace it)",
			outDir,
		)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("reading output directory %s: %w", outDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf(
			"output directory %s is not empty (use --force to replace it)", outDir,
		)
	}
	return nil
}

// copyInto copies item.src to item.dest under outDir.
func copyInto(outDir string, item copyItem) error {
	data, err := file.Read(item.src)
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
	return file.WriteAtomic(target, data)
}

// exists reports whether path names an existing regular file, treating any stat
// error (including a missing file) as absent so an optional overlay or secrets
// store is simply skipped.
func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
