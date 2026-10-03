package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/utils/filex"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

func writeSetManifest(t *testing.T) (manifestPath, dir string) {
	t.Helper()
	dir = t.TempDir()
	manifestPath = filepath.Join(dir, "envx.yaml")
	manifestBody := "environments: [development, production]\n" +
		"projects:\n  app:\n    includes: [env/postgres]\n"
	if err := os.WriteFile(manifestPath, []byte(manifestBody), 0o600); err != nil {
		t.Fatal(err)
	}

	envDir := filepath.Join(dir, "env")
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, envDir, "postgres.yaml", "host: localhost\nport: 5432\n")

	return manifestPath, dir
}

func TestOutputSet(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	disabled := false
	console := printer.New(printer.Options{
		Out:   &buf,
		Err:   &buf,
		Color: &disabled,
	})

	err := outputSet(console, env.SetResult{
		Key:         "database.password",
		OverlayPath: "/workspace/env/postgres.yaml",
	})
	if err != nil {
		t.Fatalf("outputSet: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `Set "database.password" in:`) {
		t.Errorf("output = %q, want set confirmation", got)
	}
}

func TestNewSetCommand(t *testing.T) {
	t.Parallel()

	manifest, dir := writeSetManifest(t)
	app, err := core.NewAppFactory()
	if err != nil {
		t.Fatalf("NewAppFactory(): %v", err)
	}

	svc, err := app.EnvService(manifest)
	if err != nil {
		t.Fatalf("EnvService(): %v", err)
	}

	factory := &mockEnvFactory{svc: svc}
	cmd := NewSetCommand(factory)
	cmd.PersistentFlags().String("config", manifest, "")
	cmd.SetArgs([]string{
		filepath.Join(dir, "env", "postgres"),
		"host",
		"prod-db.local",
		"--config", manifest,
		"--env", "production",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute(): %v", err)
	}

	targetOverlay := filepath.Join(dir, "env", "postgres.production.yaml")
	content, err := filex.Read(targetOverlay)
	if err != nil {
		t.Fatalf("Read(%s): %v", targetOverlay, err)
	}
	if !strings.Contains(string(content), "host: prod-db.local") {
		t.Errorf("overlay content = %q, want host: prod-db.local", string(content))
	}
}
