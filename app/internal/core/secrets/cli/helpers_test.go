package cli

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/app"
	"github.com/go-envx/envx/app/internal/core/secrets"
)

// managerFor composes the secrets service for the workspace at manifest.
func managerFor(t *testing.T, manifest string) *secrets.Service {
	t.Helper()
	manager, err := app.Factory{}.NewSecretsService(manifest)
	if err != nil {
		t.Fatalf("NewSecretsService(): %v", err)
	}
	return manager
}

// secretsPathFor returns the default secrets store path beside manifest.
func secretsPathFor(manifest string) string {
	return filepath.Join(filepath.Dir(manifest), "secrets.yaml")
}

// keysPathFor returns the default private-key path beside manifest.
func keysPathFor(manifest string) string {
	return filepath.Join(filepath.Dir(manifest), "envx.keys")
}
