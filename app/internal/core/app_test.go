package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/test/fixtures"
)

// TestAppValidateServiceDiagnosesEveryProject verifies the composed validate
// service resolves the workspace's projects and environments end to end.
func TestAppValidateServiceDiagnosesEveryProject(t *testing.T) {
	t.Parallel()

	service, err := NewApp().ValidateService(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("ValidateService(): %v", err)
	}

	report, err := service.Validate(validate.ValidateParams{})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(report.Findings) != 0 || report.Failed {
		t.Errorf("report = %+v, want a clean pass for the basic fixture", report)
	}
}

// TestAppValidateServiceMissingManifest verifies a missing manifest fails
// composition rather than producing an empty service.
func TestAppValidateServiceMissingManifest(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "envx.yaml")
	if _, err := NewApp().ValidateService(missing); err == nil {
		t.Fatal("ValidateService() succeeded without a manifest")
	}
}

// TestAppPackServiceWritesBundle verifies the composed pack service copies the
// workspace into a bundle.
func TestAppPackServiceWritesBundle(t *testing.T) {
	t.Parallel()

	service, err := NewApp().PackService(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("PackService(): %v", err)
	}

	out := filepath.Join(t.TempDir(), "dist")
	result, err := service.Pack(pack.PackParams{OutDir: out})
	if err != nil {
		t.Fatalf("Pack(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, result.ManifestFile)); err != nil {
		t.Errorf("bundle manifest not written: %v", err)
	}
}

// TestNewPackServiceRequiresLayout verifies composition rejects a missing layout.
func TestNewPackServiceRequiresLayout(t *testing.T) {
	t.Parallel()

	if _, err := NewPackService(nil); err == nil {
		t.Fatal("NewPackService(nil) succeeded")
	}
}

// TestNewValidateServiceRequiresWorkspace verifies composition rejects a missing
// resolved workspace.
func TestNewValidateServiceRequiresWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := newValidateService(nil); err == nil {
		t.Fatal("newValidateService(nil) succeeded")
	}
}
