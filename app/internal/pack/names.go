package pack

import (
	"fmt"
	"path/filepath"
	"strings"
)

// maxFilenameLen is the per-component filename limit pack keeps every produced
// directory and file name within. 255 bytes is the ceiling on ext4, APFS, NTFS
// and most other modern filesystems, so a bundle produced on one platform stays
// valid on the others. Source filenames already satisfy it (they exist on disk),
// so only a pathological project name or disambiguation suffix can approach it;
// pack errors rather than emit an invalid name.
const maxFilenameLen = 255

// includeFiles is one namespace's contribution to a project directory: the
// manifest include string it came from and the source files it resolves to (a
// base file and the selected environments' overlays).
type includeFiles struct {
	// rel is the include exactly as written in the manifest (e.g. "env/postgres"),
	// used both to read the source files and to rewrite the manifest.
	rel string
	// base is the absolute base file path, or "" when the namespace has no base.
	base string
	// overlays are the existing selected environments' overlay files.
	overlays []overlayFile
}

// overlayFile pairs one environment with the absolute path of its overlay file.
type overlayFile struct {
	// env is the environment whose overlay this is.
	env string
	// src is the absolute source path of the overlay file.
	src string
}

// maxSuffix returns the longest trailing "<.env>.yaml" any of the namespace's
// files needs, so name assignment can check the longest filename a stem produces
// against maxFilenameLen. The base file needs ".yaml"; each overlay needs
// ".<env>.yaml".
func (f includeFiles) maxSuffix() int {
	longest := 0
	if f.base != "" {
		longest = len(".yaml")
	}
	for _, overlay := range f.overlays {
		if n := len("." + overlay.env + ".yaml"); n > longest {
			longest = n
		}
	}
	return longest
}

// projectBundle is one selected project's contribution to the bundle: its
// manifest name, the sanitized directory the project's files are written into,
// the namespaces it copies there, and the base filename stem each namespace is
// written under within that directory. Because every project owns its directory,
// a namespace shared by two projects is copied into both — the bundle keeps no
// cross-project shared-file concept.
type projectBundle struct {
	// name is the manifest project name, reported back to the caller unchanged.
	name string
	// dir is the bundle-relative directory the project's files live in.
	dir string
	// includes are the namespaces this project contributes, in include order.
	includes []includeFiles
	// names maps each include's rel string to its base filename stem within dir
	// (the filename without its ".<env>.yaml" suffix).
	names map[string]string
}

// assignProjectDirs maps each selected project's manifest name to a unique,
// filesystem-safe bundle directory name. Names in reserved (the bundle's manifest
// and secrets store names and stems) are treated as already taken so a project
// directory can never collide with a root bundle file, and two projects whose
// names sanitize to the same string are disambiguated with "-2", "-3", … in the
// selection's order, so the result is deterministic.
func assignProjectDirs(
	projects []Project, reserved []string,
) (map[string]string, error) {
	used := make(map[string]bool, len(projects)+len(reserved))
	for _, name := range reserved {
		used[name] = true
	}

	dirs := make(map[string]string, len(projects))
	for _, project := range projects {
		base := sanitizeDirName(project.Name)
		candidate := base
		for n := 2; used[candidate]; n++ {
			candidate = fmt.Sprintf("%s-%d", base, n)
		}
		if len(candidate) > maxFilenameLen {
			return nil, fmt.Errorf(
				"cannot pack project %q: its bundle directory name exceeds the"+
					" %d-character filename limit",
				project.Name, maxFilenameLen,
			)
		}
		used[candidate] = true
		dirs[project.Name] = candidate
	}
	return dirs, nil
}

// assignProjectNames maps each of one project's namespaces to a base filename
// stem within that project's directory. The stem is the include's final segment
// (e.g. "env/postgres" -> "postgres"); two namespaces in the same project that
// share a final segment are disambiguated with "-2", "-3", … in include order.
// Because filenames are scoped to the project directory, no cross-project or
// reserved-name bookkeeping is needed here.
func assignProjectNames(includes []includeFiles) (map[string]string, error) {
	used := make(map[string]bool, len(includes))
	names := make(map[string]string, len(includes))
	for _, include := range includes {
		base := filepath.Base(include.rel)
		candidate := base
		for n := 1; ; n++ {
			if n > 1 {
				candidate = fmt.Sprintf("%s-%d", base, n)
			}
			if len(candidate)+include.maxSuffix() > maxFilenameLen {
				return nil, fmt.Errorf(
					"cannot pack include %q: its bundle filename exceeds the"+
						" %d-character filename limit (an environment name is too long)",
					include.rel, maxFilenameLen,
				)
			}

			filenames := include.outputFilenames(candidate)
			collision := false
			for _, filename := range filenames {
				if used[filename] {
					collision = true
					break
				}
			}
			if collision {
				continue
			}
			for _, filename := range filenames {
				used[filename] = true
			}
			names[include.rel] = candidate
			break
		}
	}
	return names, nil
}

// outputFilenames returns every filename an include produces for a candidate
// stem, so base files and overlays cannot silently overwrite each other.
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

// sanitizeDirName converts an arbitrary project name (an unconstrained manifest
// map key) into a single filesystem-safe path component: characters outside
// [A-Za-z0-9._-] become "_", so the result never contains a path separator and a
// project can never escape the bundle root. A name that reduces to empty, "." or
// ".." falls back to "project".
func sanitizeDirName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	cleaned := b.String()
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "project"
	}
	return cleaned
}
