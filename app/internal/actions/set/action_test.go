package set

import (
	"strings"
	"testing"

	"github.com/go-envx/envx/app/pkg/yamlx"
	"gopkg.in/yaml.v3"
)

// applyToYAML parses src (an empty string models a missing/fresh file), applies
// p to the document node, and returns the re-encoded YAML so tests can assert on
// the preserved comments, order, and formatting.
func applyToYAML(t *testing.T, src string, p actionParams) string {
	t.Helper()
	doc := new(yaml.Node)
	if src != "" {
		if err := yaml.Unmarshal([]byte(src), doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}
	if err := apply(doc, p); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return marshalForTest(t, doc)
}

// marshalForTest re-encodes doc the way execute does, so formatting assertions
// match the production write path.
func marshalForTest(t *testing.T, doc *yaml.Node) string {
	t.Helper()
	out, err := yamlx.Marshal(doc, detectIndent(doc))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(out)
}

// TestApplyFlatKeyEmptyDoc verifies a flat key is written to a fresh document.
func TestApplyFlatKeyEmptyDoc(t *testing.T) {
	t.Parallel()

	got := applyToYAML(t, "", actionParams{Key: "host", Value: "localhost"})
	want := "host: localhost\n"
	if got != want {
		t.Errorf("apply() =\n%q\nwant\n%q", got, want)
	}
}

// TestApplyPreservesCommentsAndOrder verifies that adding a nested key leaves the
// document's comments and existing key order untouched.
func TestApplyPreservesCommentsAndOrder(t *testing.T) {
	t.Parallel()

	src := "# managed by envx\n" +
		"host: localhost # primary\n" +
		"port: 5432\n"

	got := applyToYAML(t, src, actionParams{Key: "credentials.password", Value: "secret"})

	for _, want := range []string{
		"# managed by envx",
		"host: localhost # primary",
		"port: 5432",
		"credentials:",
		"  password: secret",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "host:") > strings.Index(got, "credentials:") {
		t.Errorf("existing keys were reordered:\n%s", got)
	}
}

// TestApplyUpdatesExistingValueInPlace verifies overwriting a key keeps the
// surrounding comments and replaces only the value.
func TestApplyUpdatesExistingValueInPlace(t *testing.T) {
	t.Parallel()

	src := "# doc\n" +
		"password: old # inline note\n"

	got := applyToYAML(t, src, actionParams{Key: "password", Value: "new"})

	if !strings.Contains(got, "password: new") {
		t.Errorf("value not updated:\n%s", got)
	}
	if strings.Contains(got, "old") {
		t.Errorf("old value was not replaced:\n%s", got)
	}
	for _, want := range []string{"# doc", "# inline note"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing comment %q:\n%s", want, got)
		}
	}
}

// TestApplyOverwritesScalarBlockingPath verifies a scalar blocking a nested path
// is replaced with a mapping.
func TestApplyOverwritesScalarBlockingPath(t *testing.T) {
	t.Parallel()

	got := applyToYAML(t, "a: scalar\n", actionParams{Key: "a.b", Value: "value"})
	want := "a:\n  b: value\n"
	if got != want {
		t.Errorf("apply() =\n%q\nwant\n%q", got, want)
	}
}

// TestApplyQuotesAmbiguousValues verifies values that look like non-strings are
// quoted, so env values round-trip as strings.
func TestApplyQuotesAmbiguousValues(t *testing.T) {
	t.Parallel()

	got := applyToYAML(t, "", actionParams{Key: "timeout", Value: "10"})
	want := "timeout: \"10\"\n"
	if got != want {
		t.Errorf("apply() =\n%q\nwant\n%q", got, want)
	}
}

// TestApplyMatchesExistingIndent verifies a new key adopts the file's existing
// indentation width rather than a fixed default.
func TestApplyMatchesExistingIndent(t *testing.T) {
	t.Parallel()

	src := "credentials:\n    username: admin\n"
	got := applyToYAML(t, src, actionParams{Key: "credentials.password", Value: "secret"})
	want := "credentials:\n    username: admin\n    password: secret\n"
	if got != want {
		t.Errorf("apply() =\n%q\nwant\n%q", got, want)
	}
}

// TestApplyRejectsNonMappingRoot verifies a document whose root is not a mapping
// is rejected rather than clobbered.
func TestApplyRejectsNonMappingRoot(t *testing.T) {
	t.Parallel()

	doc := new(yaml.Node)
	if err := yaml.Unmarshal([]byte("- a\n- b\n"), doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := apply(doc, actionParams{Key: "x", Value: "y"}); err == nil {
		t.Fatal("expected error for non-mapping root, got nil")
	}
}

// TestApplyRefusesToOverwriteList verifies setting a key that currently holds a
// list is refused, so a user's hand-authored list is never collapsed into a
// scalar, and the document is left unchanged.
func TestApplyRefusesToOverwriteList(t *testing.T) {
	t.Parallel()

	src := "hosts:\n  - a\n  - b\n"
	doc := new(yaml.Node)
	if err := yaml.Unmarshal([]byte(src), doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := apply(doc, actionParams{Key: "hosts", Value: "x"}); err == nil {
		t.Fatal("expected error overwriting a list, got nil")
	}
	if got := marshalForTest(t, doc); got != src {
		t.Errorf("list was modified on a refused set:\n%s", got)
	}
}

// TestApplyRefusesToDescendThroughList verifies a nested set that would traverse
// through a list is refused rather than silently discarding the list.
func TestApplyRefusesToDescendThroughList(t *testing.T) {
	t.Parallel()

	doc := new(yaml.Node)
	if err := yaml.Unmarshal([]byte("db:\n  - a\n  - b\n"), doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := apply(doc, actionParams{Key: "db.password", Value: "x"}); err == nil {
		t.Fatal("expected error descending through a list, got nil")
	}
}

// TestApplyRefusesToOverwriteMapping verifies setting a key that currently holds
// a nested mapping is refused, so a populated subtree is never dropped.
func TestApplyRefusesToOverwriteMapping(t *testing.T) {
	t.Parallel()

	doc := new(yaml.Node)
	if err := yaml.Unmarshal([]byte("creds:\n  user: admin\n"), doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := apply(doc, actionParams{Key: "creds", Value: "x"}); err == nil {
		t.Fatal("expected error overwriting a mapping, got nil")
	}
}
