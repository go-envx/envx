package pack

import (
	"fmt"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

// maxFilenameLen is the per-component filename limit pack keeps flattened names
// within. 255 bytes is the ceiling on ext4, APFS, NTFS and most other modern
// filesystems, so a bundle produced on one platform stays valid on the others.
const maxFilenameLen = 255

// includeFiles is one namespace's contribution to the bundle: the manifest
// include string it came from and the source files it resolves to (a base file
// and the selected environments' overlays). It carries the longest filename
// suffix any of those files needs so flat-name assignment can keep every produced
// filename within maxFilenameLen.
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
// files needs, so flat-name assignment knows how many bytes to leave for it. The
// base file needs ".yaml"; each overlay needs ".<env>.yaml".
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

// assignFlatNames maps each namespace's include string to a unique flat stem (the
// filename without its ".<env>.yaml" suffix) placed in the bundle root. Each stem
// is the include's final segment (e.g. "env/postgres" -> "postgres"); two
// namespaces that share one are disambiguated by appending "-2", "-3", …, and a
// stem is truncated when necessary so the longest filename it produces stays
// within maxFilenameLen. Names in reserved (the manifest and secrets store stems)
// are treated as already taken so a namespace never shadows them. Assignment walks
// the namespaces in sorted include order, so the result is deterministic
// regardless of how the projects were declared.
func assignFlatNames(
	includes []includeFiles, reserved []string,
) (map[string]string, error) {
	used := make(map[string]bool, len(includes)+len(reserved))
	for _, name := range reserved {
		used[name] = true
	}

	ordered := append([]includeFiles(nil), includes...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].rel < ordered[j].rel })

	names := make(map[string]string, len(ordered))
	for _, include := range ordered {
		// Leave room for the longest "<.env>.yaml" suffix so every file this
		// namespace produces fits within the filename limit.
		maxStem := maxFilenameLen - include.maxSuffix()
		if maxStem < 1 {
			return nil, fmt.Errorf(
				"cannot flatten include %q within the %d-character filename limit"+
					" (an environment name is too long)",
				include.rel, maxFilenameLen,
			)
		}

		stem := clampBytes(filepath.Base(include.rel), maxStem)
		candidate := stem
		for n := 2; used[candidate]; n++ {
			suffix := fmt.Sprintf("-%d", n)
			candidate = clampBytes(stem, maxStem-len(suffix)) + suffix
		}

		used[candidate] = true
		names[include.rel] = candidate
	}
	return names, nil
}

// clampBytes truncates s to at most limit bytes on a rune boundary, so a
// truncated name is never invalid UTF-8. A non-positive limit yields the empty
// string.
func clampBytes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(s) <= limit {
		return s
	}
	out := make([]byte, 0, limit)
	for _, r := range s {
		if len(out)+utf8.RuneLen(r) > limit {
			break
		}
		out = utf8.AppendRune(out, r)
	}
	return string(out)
}
