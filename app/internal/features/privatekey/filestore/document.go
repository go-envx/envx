package filestore

import (
	"fmt"
	"strings"
)

// entry is one significant NAME=value line in a private-key document.
type entry struct {
	// name is the uppercased group name.
	name string
	// value is the raw private key after the first '='.
	value string
	// line is the entry's index into document.lines.
	line int
}

// document is a parsed NAME=value private-key file that preserves original line
// order and comments so read and write paths share one grammar.
type document struct {
	// lines holds every original line, including blanks and comments.
	lines []string
	// entries holds the significant NAME=value lines in file order.
	entries []entry
	// byName maps an uppercased group name to its index into entries.
	byName map[string]int
}

// parseDocument parses NAME=value content, rejecting malformed, duplicate, and
// empty entries so a present-but-broken input fails closed.
func parseDocument(content, inputName string) (document, error) {
	lines := strings.Split(content, "\n")
	parsed := document{lines: lines, byName: make(map[string]int)}
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
			return document{}, fmt.Errorf(
				"malformed private-key entry in %s at line %d", inputName, index+1,
			)
		}
		if _, duplicate := parsed.byName[name]; duplicate {
			return document{}, fmt.Errorf(
				"duplicate private-key group %q in %s", name, inputName,
			)
		}
		if strings.TrimSpace(value) == "" {
			return document{}, fmt.Errorf(
				"private key for group %q is empty in %s", name, inputName,
			)
		}
		parsed.byName[name] = len(parsed.entries)
		parsed.entries = append(
			parsed.entries, entry{name: name, value: value, line: index},
		)
	}
	return parsed, nil
}

// lookup returns the raw private key for group, matched case-insensitively.
func (d document) lookup(group string) (string, bool) {
	index, ok := d.byName[strings.ToUpper(group)]
	if !ok {
		return "", false
	}
	return d.entries[index].value, true
}

// upsert sets group's private key, updating an existing entry in place or
// appending a new one, and renders the file with a single trailing newline.
func (d document) upsert(group, privateKey string) string {
	name := strings.ToUpper(group)
	entry := name + "=" + privateKey
	switch index, ok := d.byName[name]; {
	case ok:
		d.lines[d.entries[index].line] = entry
	case len(d.lines) == 1 && d.lines[0] == "":
		d.lines[0] = entry
	default:
		// Drop the trailing newline's empty split element so the new entry does
		// not land after a blank separator line.
		for len(d.lines) > 0 && d.lines[len(d.lines)-1] == "" {
			d.lines = d.lines[:len(d.lines)-1]
		}
		d.lines = append(d.lines, entry)
	}
	return strings.TrimRight(strings.Join(d.lines, "\n"), "\n") + "\n"
}
