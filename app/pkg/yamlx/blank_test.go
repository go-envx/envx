package yamlx

import "testing"

// TestPreserveBlankLinesReinsertsSeparators verifies blank runs from the original
// are restored before the matching content lines in the rendered output.
func TestPreserveBlankLinesReinsertsSeparators(t *testing.T) {
	t.Parallel()

	original := "public_keys:\n  dev: aaa\n\n  shared: bbb\n\n" +
		"secrets:\n  dev:\n    key: ccc\n"
	rendered := "public_keys:\n  dev: aaa\n  shared: bbb\n" +
		"secrets:\n  dev:\n    key: ccc\n"

	got := string(PreserveBlankLines([]byte(original), []byte(rendered)))
	if got != original {
		t.Errorf("PreserveBlankLines() =\n%q\nwant\n%q", got, original)
	}
}

// TestPreserveBlankLinesKeepsConsecutiveRuns verifies a run of multiple blank
// lines is restored in full.
func TestPreserveBlankLinesKeepsConsecutiveRuns(t *testing.T) {
	t.Parallel()

	original := "a: 1\n\n\nb: 2\n"
	rendered := "a: 1\nb: 2\n"

	got := string(PreserveBlankLines([]byte(original), []byte(rendered)))
	if got != original {
		t.Errorf("PreserveBlankLines() =\n%q\nwant\n%q", got, original)
	}
}

// TestPreserveBlankLinesDropsRemovedAnchors verifies a blank line attached to a
// line that no longer exists is not reinserted, while separators before surviving
// lines are preserved.
func TestPreserveBlankLinesDropsRemovedAnchors(t *testing.T) {
	t.Parallel()

	original := "a: 1\n\nb: 2\n\nc: 3\n"
	rendered := "a: 1\nc: 3\n"
	want := "a: 1\n\nc: 3\n"

	got := string(PreserveBlankLines([]byte(original), []byte(rendered)))
	if got != want {
		t.Errorf("PreserveBlankLines() =\n%q\nwant\n%q", got, want)
	}
}

// TestPreserveBlankLinesIgnoresAddedLines verifies content added during the edit
// receives no separator and does not disturb later matches.
func TestPreserveBlankLinesIgnoresAddedLines(t *testing.T) {
	t.Parallel()

	original := "a: 1\n\nb: 2\n"
	rendered := "a: 1\nnew: 9\nb: 2\n"
	want := "a: 1\nnew: 9\n\nb: 2\n"

	got := string(PreserveBlankLines([]byte(original), []byte(rendered)))
	if got != want {
		t.Errorf("PreserveBlankLines() =\n%q\nwant\n%q", got, want)
	}
}

// TestPreserveBlankLinesPreservesLeadingBlankLines verifies blank lines before
// the first content line are restored.
func TestPreserveBlankLinesPreservesLeadingBlankLines(t *testing.T) {
	t.Parallel()

	original := "\na: 1\n"
	rendered := "a: 1\n"

	got := string(PreserveBlankLines([]byte(original), []byte(rendered)))
	if got != original {
		t.Errorf("PreserveBlankLines() =\n%q\nwant\n%q", got, original)
	}
}

// TestPreserveBlankLinesReturnsRenderedWhenOriginalEmpty verifies a missing or
// empty original leaves the rendered output untouched.
func TestPreserveBlankLinesReturnsRenderedWhenOriginalEmpty(t *testing.T) {
	t.Parallel()

	rendered := []byte("a: 1\nb: 2\n")
	got := string(PreserveBlankLines(nil, rendered))
	if got != string(rendered) {
		t.Errorf("PreserveBlankLines() = %q, want %q", got, rendered)
	}
}

// TestPreserveBlankLinesNoOpWithoutBlankLines verifies a source with no blank
// lines yields the rendered output unchanged.
func TestPreserveBlankLinesNoOpWithoutBlankLines(t *testing.T) {
	t.Parallel()

	original := []byte("a: 1\nb: 2\n")
	rendered := []byte("a: 1\nb: 2\n")
	got := string(PreserveBlankLines(original, rendered))
	if got != string(rendered) {
		t.Errorf("PreserveBlankLines() = %q, want %q", got, rendered)
	}
}
