package keyring

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

// versionSeparator joins a group and a public-key fingerprint in a retained
// entry name. Group names cannot contain it.
const versionSeparator = "/"

// entry is one significant NAME[/VERSION]=value line.
type entry struct {
	// group is the uppercase group name.
	group string
	// version is the public-key fingerprint; empty for a legacy entry.
	version string
	// value is the private key.
	value string
	// line is the entry's index in document.lines.
	line int
}

// document is a parsed key file or ENVX_PRIVATE_KEY bundle. It preserves
// original lines and comments for rewriting.
type document struct {
	// lines holds every original line, including comments and blanks.
	lines []string
	// entries holds the parsed key lines in file order.
	entries []entry
}

// parseDocument parses NAME=value lines, rejecting malformed, duplicate, and
// empty entries. Unversioned entries are the legacy format; versioned entries
// are indexed by the fingerprint of their public key.
func parseDocument(content, source string) (document, error) {
	doc := document{lines: strings.Split(content, "\n")}
	seen := make(map[string]bool)
	for index, raw := range doc.lines {
		line := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		name = strings.ToUpper(strings.TrimSpace(name))
		group, version, versioned := strings.Cut(name, versionSeparator)
		if !ok || group == "" || (versioned && version == "") {
			return document{}, fmt.Errorf(
				"malformed private-key entry in %s at line %d", source, index+1,
			)
		}
		if seen[name] {
			return document{}, fmt.Errorf(
				"duplicate private-key entry in %s at line %d", source, index+1,
			)
		}
		seen[name] = true
		if strings.TrimSpace(value) == "" {
			return document{}, fmt.Errorf(
				"empty private-key entry in %s at line %d", source, index+1,
			)
		}
		doc.entries = append(doc.entries, entry{
			group: group, version: version, value: value, line: index,
		})
	}
	return doc, nil
}

// lookup selects a group's key. A versioned entry matching publicKey wins,
// then the legacy entry, whose identity is unknown. Without a public key,
// selection must be unambiguous.
func (d document) lookup(group, publicKey string) (key string, found bool, err error) {
	var legacy []entry
	var versions []entry
	for _, e := range d.entries {
		switch {
		case e.group != group:
		case e.version == "":
			legacy = append(legacy, e)
		default:
			versions = append(versions, e)
		}
	}
	if publicKey == "" {
		candidates := slices.Concat(legacy, versions)
		switch len(candidates) {
		case 0:
			return "", false, nil
		case 1:
			return candidates[0].value, true, nil
		default:
			return "", false, fmt.Errorf(
				"%d private keys are stored for group %q and no public key "+
					"identifies which to use", len(candidates), group,
			)
		}
	}
	id := fingerprint(publicKey)
	for _, e := range versions {
		if e.version == id {
			return e.value, true, nil
		}
	}
	if len(legacy) == 1 {
		return legacy[0].value, true, nil
	}
	return "", false, nil
}

// retain adds or replaces the versioned entry for publicKey, leaving every
// other entry untouched, and renders the file with one trailing newline.
func (d document) retain(group, publicKey, privateKey string) string {
	id := fingerprint(publicKey)
	line := group + versionSeparator + id + "=" + privateKey
	lines := append([]string(nil), d.lines...)
	for _, e := range d.entries {
		if e.group == group && e.version == id {
			lines[e.line] = line
			return render(lines)
		}
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return render(append(lines, line))
}

// render joins lines with exactly one trailing newline.
func render(lines []string) string {
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

// fingerprint is a short, stable public-key identity for entry names.
func fingerprint(publicKey string) string {
	sum := sha256.Sum256([]byte(publicKey))
	return strings.ToUpper(hex.EncodeToString(sum[:8]))
}
