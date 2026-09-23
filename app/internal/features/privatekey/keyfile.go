package privatekey

import (
	"fmt"
	"strings"
)

// keyEntry is one significant NAME=value line in a private-key file.
type keyEntry struct {
	// name is the uppercased group name.
	name string
	// value is the raw private key after the first '='.
	value string
	// line is the entry's index into keyFile.lines.
	line int
}

// keyFile is a parsed NAME=value private-key file that preserves original line
// order and comments so read and write paths share one grammar.
type keyFile struct {
	// lines holds every original line, including blanks and comments.
	lines []string
	// entries holds the significant NAME=value lines in file order.
	entries []keyEntry
	// byName maps an uppercased group name to its index into entries.
	byName map[string]int
}

// parseKeyFile parses NAME=value content, rejecting malformed, duplicate, and
// empty entries so a present-but-broken input fails closed.
func parseKeyFile(content, inputName string) (keyFile, error) {
	lines := strings.Split(content, "\n")
	parsed := keyFile{lines: lines, byName: make(map[string]int)}
	for index, raw := range lines {
		// Strip \r before parsing so CRLF values do not retain a trailing return.
		line := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		name, value, ok := strings.Cut(line, "=")
		name = strings.ToUpper(strings.TrimSpace(name))
		if !ok || name == "" {
			return keyFile{}, fmt.Errorf(
				"malformed private-key entry in %s at line %d", inputName, index+1,
			)
		}
		if _, duplicate := parsed.byName[name]; duplicate {
			return keyFile{}, fmt.Errorf(
				"duplicate private-key group %q in %s", name, inputName,
			)
		}
		if strings.TrimSpace(value) == "" {
			return keyFile{}, fmt.Errorf(
				"private key for group %q is empty in %s", name, inputName,
			)
		}
		parsed.byName[name] = len(parsed.entries)
		parsed.entries = append(
			parsed.entries, keyEntry{name: name, value: value, line: index},
		)
	}
	return parsed, nil
}

// lookup returns the raw private key for group, matched case-insensitively.
func (f keyFile) lookup(group string) (string, bool) {
	index, ok := f.byName[strings.ToUpper(group)]
	if !ok {
		return "", false
	}
	return f.entries[index].value, true
}

// upsert sets group's private key, updating an existing entry in place or
// appending a new one, and renders the file with a single trailing newline.
func (f keyFile) upsert(group, privateKey string) string {
	name := strings.ToUpper(group)
	entry := name + "=" + privateKey
	switch index, ok := f.byName[name]; {
	case ok:
		f.lines[f.entries[index].line] = entry
	case len(f.lines) == 1 && f.lines[0] == "":
		f.lines[0] = entry
	default:
		// Drop the trailing newline's empty split element so the new entry does
		// not land after a blank separator line.
		for len(f.lines) > 0 && f.lines[len(f.lines)-1] == "" {
			f.lines = f.lines[:len(f.lines)-1]
		}
		f.lines = append(f.lines, entry)
	}
	return strings.TrimRight(strings.Join(f.lines, "\n"), "\n") + "\n"
}
