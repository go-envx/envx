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
}

// document is a parsed key file or ENVX_PRIVATE_KEY bundle.
type document struct {
	// entries holds the parsed key lines in file order.
	entries []entry
}

// parseDocument parses NAME=value lines, rejecting malformed, conflicting
// duplicate, and empty entries. An exact duplicate line, which concurrent
// Retain calls can append, is accepted. Unversioned entries are the legacy
// format; versioned entries are indexed by the fingerprint of their public key.
func parseDocument(content, source string) (document, error) {
	var doc document
	seen := make(map[string]string)
	for index, raw := range strings.Split(content, "\n") {
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
		if strings.TrimSpace(value) == "" {
			return document{}, fmt.Errorf(
				"empty private-key entry in %s at line %d", source, index+1,
			)
		}
		if previous, dup := seen[name]; dup {
			if previous != value {
				return document{}, fmt.Errorf(
					"duplicate private-key entry in %s at line %d", source, index+1,
				)
			}
			continue
		}
		seen[name] = value
		doc.entries = append(doc.entries, entry{
			group: group, version: version, value: value,
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

// has reports whether the document holds a versioned entry for the group and
// public key.
func (d document) has(group, publicKey string) bool {
	id := fingerprint(publicKey)
	return slices.ContainsFunc(d.entries, func(e entry) bool {
		return e.group == group && e.version == id
	})
}

// fingerprint is a short, stable public-key identity for entry names.
func fingerprint(publicKey string) string {
	sum := sha256.Sum256([]byte(publicKey))
	return strings.ToUpper(hex.EncodeToString(sum[:8]))
}
