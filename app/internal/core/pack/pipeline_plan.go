package pack

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/lib/filex"
)

// maxFilenameLen is the per-component filename limit on ext4, APFS, NTFS, and
// most other filesystems; pack errors rather than produce a longer name.
const maxFilenameLen = 255

// includeFiles is one namespace's source files within a project.
type includeFiles struct {
	// rel is the include as written in the manifest.
	rel string
	// base is the absolute base file path, or empty without a base file.
	base string
	// overlays are the selected environments' existing overlay files.
	overlays []overlayFile
}

// overlayFile pairs an environment with its overlay file.
type overlayFile struct {
	// env is the overlay's environment.
	env string
	// src is the absolute overlay file path.
	src string
}

// projectBundle is one selected project's contribution to the bundle.
type projectBundle struct {
	// name is the manifest project name.
	name string
	// dir is the bundle-relative project directory.
	dir string
	// includes are the project's namespaces in include order.
	includes []includeFiles
	// names maps each include's rel to its filename stem within dir.
	names map[string]string
}

// copyItem pairs a source file with its bundle-relative destination.
type copyItem struct {
	// src is the absolute source path.
	src string
	// dest is the slash-separated bundle-relative destination.
	dest string
	// data is the source content.
	data []byte
}

// planProjects discovers each project's files and assigns their names within
// the project's directory. A namespace two projects include is planned twice.
func planProjects(
	projects []project, environments []string, dirs map[string]string,
) ([]projectBundle, error) {
	bundles := make([]projectBundle, 0, len(projects))
	for _, p := range projects {
		includes, err := discoverIncludes(p, environments)
		if err != nil {
			return nil, err
		}
		names, err := assignProjectNames(includes)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, projectBundle{
			name:     p.name,
			dir:      dirs[p.name],
			includes: includes,
			names:    names,
		})
	}
	return bundles, nil
}

// discoverIncludes collects each distinct include's base file and selected
// overlays. An include matching no file fails so a typo cannot ship silently.
func discoverIncludes(p project, environments []string) ([]includeFiles, error) {
	seen := make(map[string]struct{}, len(p.includes))
	var includes []includeFiles
	for _, inc := range p.includes {
		if _, ok := seen[inc.rel]; ok {
			continue
		}
		seen[inc.rel] = struct{}{}

		files := includeFiles{rel: inc.rel}
		if base := inc.path + ".yaml"; exists(base) {
			files.base = base
		}
		for _, env := range environments {
			if overlay := inc.path + "." + env + ".yaml"; exists(overlay) {
				files.overlays = append(files.overlays, overlayFile{env: env, src: overlay})
			}
		}
		if files.base == "" && len(files.overlays) == 0 {
			return nil, fmt.Errorf(
				"include %q in project %q matches no base file or selected overlay",
				inc.rel, p.name,
			)
		}
		includes = append(includes, files)
	}
	return includes, nil
}

// planFiles lists and reads every namespace file as "<dir>/<stem>.yaml" or
// "<dir>/<stem>.<env>.yaml", reading each source once so references are
// scanned from exactly the staged bytes. The secrets store is filtered, not
// copied.
func planFiles(bundles []projectBundle) ([]copyItem, error) {
	cache := make(map[string][]byte)
	var items []copyItem
	add := func(src, dest string) error {
		data, ok := cache[src]
		if !ok {
			var err error
			if data, err = filex.Read(src); err != nil {
				return fmt.Errorf("reading %s: %w", src, err)
			}
			cache[src] = data
		}
		items = append(items, copyItem{src: src, dest: dest, data: data})
		return nil
	}
	for _, bundle := range bundles {
		for _, inc := range bundle.includes {
			prefix := bundle.dir + "/" + bundle.names[inc.rel]
			if inc.base != "" {
				if err := add(inc.base, prefix+".yaml"); err != nil {
					return nil, err
				}
			}
			for _, overlay := range inc.overlays {
				if err := add(overlay.src, prefix+"."+overlay.env+".yaml"); err != nil {
					return nil, err
				}
			}
		}
	}
	return items, nil
}

// assignProjectDirs maps each project to a unique, filesystem-safe directory
// name that avoids reserved root names, suffixing "-2", "-3", … in order.
func assignProjectDirs(
	projects []project, reserved []string,
) (map[string]string, error) {
	used := make(map[string]bool, len(projects)+len(reserved))
	for _, name := range reserved {
		used[name] = true
	}
	dirs := make(map[string]string, len(projects))
	for _, p := range projects {
		base := sanitizeDirName(p.name)
		candidate := base
		for n := 2; used[candidate]; n++ {
			candidate = fmt.Sprintf("%s-%d", base, n)
		}
		if len(candidate) > maxFilenameLen {
			return nil, fmt.Errorf(
				"cannot pack project %q: its bundle directory name exceeds the"+
					" %d-character filename limit",
				p.name, maxFilenameLen,
			)
		}
		used[candidate] = true
		dirs[p.name] = candidate
	}
	return dirs, nil
}

// assignProjectNames maps each namespace to a filename stem from its final
// segment, suffixing "-2", "-3", … in include order. Stems must be unique on
// their own, since a shared stem would merge two includes in the rewritten
// manifest, and their files must not collide when environment names hold dots.
func assignProjectNames(includes []includeFiles) (map[string]string, error) {
	usedStems := make(map[string]bool, len(includes))
	usedFiles := make(map[string]bool, len(includes))
	names := make(map[string]string, len(includes))
	for _, inc := range includes {
		base := filepath.Base(inc.rel)
		candidate := base
		var filenames []string
		for n := 2; ; n++ {
			filenames = inc.outputFilenames(candidate)
			if slices.ContainsFunc(filenames, func(f string) bool {
				return len(f) > maxFilenameLen
			}) {
				return nil, fmt.Errorf(
					"cannot pack include %q: its bundle filename exceeds the"+
						" %d-character filename limit (an environment name is too long)",
					inc.rel, maxFilenameLen,
				)
			}
			if !usedStems[candidate] &&
				!slices.ContainsFunc(filenames, func(f string) bool { return usedFiles[f] }) {
				break
			}
			candidate = fmt.Sprintf("%s-%d", base, n)
		}
		usedStems[candidate] = true
		for _, filename := range filenames {
			usedFiles[filename] = true
		}
		names[inc.rel] = candidate
	}
	return names, nil
}

// outputFilenames returns every filename the namespace produces for stem.
func (f includeFiles) outputFilenames(stem string) []string {
	filenames := make([]string, 0, 1+len(f.overlays))
	if f.base != "" {
		filenames = append(filenames, stem+".yaml")
	}
	for _, overlay := range f.overlays {
		filenames = append(filenames, stem+"."+overlay.env+".yaml")
	}
	return filenames
}

// sanitizeDirName keeps [A-Za-z0-9-] and collapses every other run of
// characters, including "_", "." and "/", into one "_", so a project cannot
// escape the bundle root. A name that sanitizes to empty becomes "project".
func sanitizeDirName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prevUnderscore := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-':
			b.WriteRune(r)
			prevUnderscore = false
		case !prevUnderscore:
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	if b.Len() == 0 {
		return "project"
	}
	return b.String()
}
