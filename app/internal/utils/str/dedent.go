package str

import (
	"math"
	"strings"
)

// Dedent removes the common leading whitespace from all non-empty lines in s.
// Tabs are expanded to spaces first (so Go-indented raw strings work with
// tab-sensitive formats like YAML). Leading and trailing blank lines are
// trimmed. An optional second argument specifies the number of spaces to
// preserve in front of each non-empty line after dedenting.
func Dedent(s string, indent ...int) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	s = strings.TrimLeft(s, "\n")
	s = strings.TrimRight(s, " \n")

	lines := strings.Split(s, "\n")
	minIndent := math.MaxInt
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		col := len(line) - len(strings.TrimLeft(line, " "))
		if col < minIndent {
			minIndent = col
		}
	}

	if minIndent != 0 && minIndent != math.MaxInt {
		for i, line := range lines {
			if len(line) >= minIndent {
				lines[i] = line[minIndent:]
			}
		}
	}

	preserve := 0
	if len(indent) > 0 && indent[0] > 0 {
		preserve = indent[0]
	}
	if preserve > 0 {
		prefix := strings.Repeat(" ", preserve)
		for i, line := range lines {
			if line != "" {
				lines[i] = prefix + line
			}
		}
	}

	return strings.Join(lines, "\n")
}
