package fixtures_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
)

func TestTestdata(t *testing.T) {
	t.Parallel()

	base := fixtures.Testdata()
	info, err := os.Stat(base)
	if err != nil {
		t.Fatalf("Testdata() path %q stat error: %v", base, err)
	}
	if !info.IsDir() {
		t.Fatalf("Testdata() path %q is not a directory", base)
	}
	if !strings.HasSuffix(filepath.ToSlash(base), "app/test/testdata") {
		t.Errorf("Testdata() = %q, expected path ending in app/test/testdata", base)
	}

	basicDir := fixtures.Testdata("basic")
	info, err = os.Stat(basicDir)
	if err != nil {
		t.Fatalf("Testdata(\"basic\") path %q stat error: %v", basicDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("Testdata(\"basic\") path %q is not a directory", basicDir)
	}
}

func TestManifest(t *testing.T) {
	t.Parallel()

	manifestPath := fixtures.Manifest("basic")
	info, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatalf("Manifest(\"basic\") path %q stat error: %v", manifestPath, err)
	}
	if info.IsDir() {
		t.Fatalf("Manifest(\"basic\") path %q is a directory, expected file", manifestPath)
	}
	expectedSuffix := filepath.Join("app", "test", "testdata", "basic", "envx.yaml")
	if !strings.HasSuffix(manifestPath, expectedSuffix) {
		t.Errorf(
			"Manifest(\"basic\") = %q, expected suffix %q",
			manifestPath,
			expectedSuffix,
		)
	}
}
