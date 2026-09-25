package filestore

import "testing"

// TestParseDocumentValid verifies comments, blank lines, CRLF endings, and
// case-insensitive lookup across a well-formed key file.
func TestParseDocumentValid(t *testing.T) {
	t.Parallel()

	content := "# comment\r\n\nPRODUCTION=prod-value\r\nSHARED=shared-value\n"
	parsed, err := parseDocument(content, "envx.keys")
	if err != nil {
		t.Fatalf("parseDocument(): %v", err)
	}
	tests := []struct {
		group string
		want  string
		found bool
	}{
		{group: "production", want: "prod-value", found: true},
		{group: "PRODUCTION", want: "prod-value", found: true},
		{group: "shared", want: "shared-value", found: true},
		{group: "missing", want: "", found: false},
	}
	for _, tt := range tests {
		got, found := parsed.lookup(tt.group)
		if got != tt.want || found != tt.found {
			t.Errorf(
				"lookup(%q) = (%q, %t), want (%q, %t)",
				tt.group, got, found, tt.want, tt.found,
			)
		}
	}
}

// TestParseDocumentRejectsMalformed verifies the parser fails closed on malformed,
// empty, and duplicate entries instead of silently skipping them.
func TestParseDocumentRejectsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "no separator", content: "PRODUCTION"},
		{name: "empty name", content: "=value"},
		{name: "whitespace name", content: "   =value"},
		{name: "empty value", content: "PRODUCTION="},
		{name: "whitespace value", content: "PRODUCTION=   "},
		{name: "duplicate group", content: "PRODUCTION=one\nproduction=two"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseDocument(tt.content, "envx.keys"); err == nil {
				t.Fatalf("parseDocument(%q) accepted malformed input", tt.content)
			}
		})
	}
}

// TestDocumentUpsertUpdatesInPlace verifies updating an existing group preserves
// comments, blank lines, and the order of surrounding entries.
func TestDocumentUpsertUpdatesInPlace(t *testing.T) {
	t.Parallel()

	content := "# header\nPRODUCTION=old-value\nSHARED=shared-value\n"
	parsed, err := parseDocument(content, "envx.keys")
	if err != nil {
		t.Fatalf("parseDocument(): %v", err)
	}
	got := parsed.upsert("production", "new-value")
	want := "# header\nPRODUCTION=new-value\nSHARED=shared-value\n"
	if got != want {
		t.Errorf("upsert() = %q, want %q", got, want)
	}
}

// TestDocumentUpsertAppendsNewGroup verifies a new group is appended after the
// existing entries with a single trailing newline.
func TestDocumentUpsertAppendsNewGroup(t *testing.T) {
	t.Parallel()

	parsed, err := parseDocument("PRODUCTION=prod-value\n", "envx.keys")
	if err != nil {
		t.Fatalf("parseDocument(): %v", err)
	}
	got := parsed.upsert("shared", "shared-value")
	want := "PRODUCTION=prod-value\nSHARED=shared-value\n"
	if got != want {
		t.Errorf("upsert() = %q, want %q", got, want)
	}
}

// TestDocumentUpsertWritesEmptyFile verifies upserting into empty content yields a
// single entry rather than a leading blank line.
func TestDocumentUpsertWritesEmptyFile(t *testing.T) {
	t.Parallel()

	parsed, err := parseDocument("", "envx.keys")
	if err != nil {
		t.Fatalf("parseDocument(): %v", err)
	}
	got := parsed.upsert("production", "prod-value")
	if got != "PRODUCTION=prod-value\n" {
		t.Errorf("upsert() = %q, want single entry", got)
	}
}

// TestDocumentUpsertPreservesUntouchedLineEndings verifies a case-insensitive
// update rewrites only the matched entry and leaves other lines, including their
// CRLF endings, intact.
func TestDocumentUpsertPreservesUntouchedLineEndings(t *testing.T) {
	t.Parallel()

	content := "PRODUCTION=old-value\r\nSHARED=shared-value\r\n"
	parsed, err := parseDocument(content, "envx.keys")
	if err != nil {
		t.Fatalf("parseDocument(): %v", err)
	}
	got := parsed.upsert("Production", "new-value")
	want := "PRODUCTION=new-value\nSHARED=shared-value\r\n"
	if got != want {
		t.Errorf("upsert() = %q, want %q", got, want)
	}
}
