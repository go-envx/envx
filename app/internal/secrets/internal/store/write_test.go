package store

import (
	"os"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestWrites verifies additions, updates, deletion, and retention of an
// empty group after its last secret is removed.
func TestWrites(t *testing.T) {
	t.Parallel()

	path := writeDocument(t, "secrets:\n  production:\n    first: one\n    second: two\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := document.SetPublicKey("production", "age-public-key:age1prod"); err != nil {
		t.Fatalf("SetPublicKey() add error = %v", err)
	}
	if err := document.SetPublicKey(
		"PRODUCTION", "age-public-key:age1updated",
	); err != nil {
		t.Fatalf("SetPublicKey() update error = %v", err)
	}
	if err := document.SetSecret("PRODUCTION", "first", "updated"); err != nil {
		t.Fatalf("SetSecret() update error = %v", err)
	}
	if err := document.SetSecret("shared", "token", "value"); err != nil {
		t.Fatalf("SetSecret() group add error = %v", err)
	}
	if err := document.SetSecret("production", "third", "three"); err != nil {
		t.Fatalf("SetSecret() entry add error = %v", err)
	}
	if deleted, err := document.DeleteSecret(
		"production", "second",
	); err != nil || !deleted {
		t.Fatalf("DeleteSecret() = %v, %v; want true, nil", deleted, err)
	}
	if deleted, err := document.DeleteSecret(
		"production", "missing",
	); err != nil || deleted {
		t.Fatalf("DeleteSecret() missing = %v, %v; want false, nil", deleted, err)
	}

	if err := document.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := Open(path)
	if err != nil {
		t.Fatalf("Open() after Save error = %v", err)
	}
	if got, ok := reloaded.PublicKey("production"); !ok ||
		got != "age-public-key:age1updated" {
		t.Errorf("PublicKey() after Save = %q, %v", got, ok)
	}
	if got, ok := reloaded.Secret("production", "first"); !ok || got.Value != "updated" {
		t.Errorf("Secret() after Save = %#v, %v", got, ok)
	}
	if _, ok := reloaded.Secret("production", "second"); ok {
		t.Error("deleted secret is still present")
	}
	if _, ok := reloaded.Secret("shared", "token"); !ok {
		t.Error("new group secret is absent")
	}
}

// -------------------------------------------------------------------------------------

// TestCommentAndOrderPreservation verifies node mutation keeps comments and
// leaves existing entries in their original order.
func TestCommentAndOrderPreservation(t *testing.T) {
	t.Parallel()

	path := writeDocument(t, "# workspace secrets\n"+
		"secrets:\n"+
		"  production:\n"+
		"    # keep this comment\n"+
		"    first: one\n"+
		"    second: two\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := document.SetSecret("production", "first", "updated"); err != nil {
		t.Fatalf("SetSecret() update error = %v", err)
	}
	if err := document.SetSecret("production", "third", "three"); err != nil {
		t.Fatalf("SetSecret() add error = %v", err)
	}
	if err := document.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	//nolint:gosec // G304: path is created inside this test's temporary directory.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	if !strings.Contains(output, "# keep this comment") {
		t.Fatalf("saved document lost comment:\n%s", output)
	}
	first := strings.Index(output, "first: updated")
	second := strings.Index(output, "second: two")
	third := strings.Index(output, "third: three")
	if first < 0 || second < 0 || third < 0 || first > second || second > third {
		t.Fatalf("saved document order is wrong:\n%s", output)
	}
}
