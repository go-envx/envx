package yamlx

import "strings"

// PreserveBlankLines reinserts the blank lines from original into rendered.
//
// The YAML node model does not retain blank lines, so re-encoding an edited node
// tree collapses the separators an author placed between entries. This restores
// them heuristically: each blank line is treated as belonging to the next
// content line, and blank runs are reinserted before the matching line in the
// rendered output. Lines are matched by their trimmed text in document order, so
// added lines gain no separator and removed lines drop theirs. When original is
// empty, rendered is returned unchanged.
func PreserveBlankLines(original, rendered []byte) []byte {
	if len(original) == 0 {
		return rendered
	}

	markers := blankMarkers(original)
	if len(markers) == 0 {
		return rendered
	}

	renderedLines := strings.Split(string(rendered), "\n")
	out := make([]string, 0, len(renderedLines))
	cursor := 0
	for _, line := range renderedLines {
		trimmed := strings.TrimSpace(line)
		if index := matchMarker(markers, cursor, trimmed); index >= 0 {
			for range markers[index].blankBefore {
				out = append(out, "")
			}
			cursor = index + 1
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}

// blankMarker records one content line and the blank lines that preceded it.
type blankMarker struct {
	// text is the trimmed content of the line.
	text string
	// blankBefore counts the blank lines immediately preceding the line.
	blankBefore int
}

// blankMarkers reduces original to its content lines, each carrying the count of
// blank lines that immediately preceded it.
func blankMarkers(original []byte) []blankMarker {
	var markers []blankMarker
	blank := 0
	for _, line := range strings.Split(string(original), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			blank++
			continue
		}
		markers = append(markers, blankMarker{text: trimmed, blankBefore: blank})
		blank = 0
	}
	return markers
}

// matchMarker returns the index of the first marker at or after cursor whose text
// equals trimmed, or -1 when none matches. An empty trimmed line never matches so
// blank runs are anchored only to real content.
func matchMarker(markers []blankMarker, cursor int, trimmed string) int {
	if trimmed == "" {
		return -1
	}
	for index := cursor; index < len(markers); index++ {
		if markers[index].text == trimmed {
			return index
		}
	}
	return -1
}
