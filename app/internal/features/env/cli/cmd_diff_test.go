package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

func writeDiffManifest(t *testing.T) (manifestPath, dir string) {
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
	writeYAML(t, envDir, "postgres.development.yaml", "host: dev-db.local\n")
	writeYAML(t, envDir, "postgres.production.yaml", "host: prod-db.internal\n")

	return manifestPath, dir
}

func TestNewDiffCommandTable(t *testing.T) {
	t.Parallel()

	manifest, _ := writeDiffManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	app := core.NewApp()

	svc, err := app.EnvService(input, "app")
	if err != nil {
		t.Fatalf("EnvService(): %v", err)
	}

	factory := &mockEnvFactory{svc: svc}
	cmd := NewDiffCommand(factory)
	cmd.PersistentFlags().String("config", manifest, "")
	cmd.SetArgs([]string{
		"app",
		"development",
		"production",
		"--config", manifest,
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute(): %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "HOST") ||
		!strings.Contains(output, "dev-db.local -> prod-db.internal") {
		t.Errorf("output = %q, want HOST diff", output)
	}
}

func TestNewDiffCommandJSON(t *testing.T) {
	t.Parallel()

	manifest, _ := writeDiffManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	app := core.NewApp()

	svc, err := app.EnvService(input, "app")
	if err != nil {
		t.Fatalf("EnvService(): %v", err)
	}

	factory := &mockEnvFactory{svc: svc}
	cmd := NewDiffCommand(factory)
	cmd.PersistentFlags().String("config", manifest, "")
	cmd.SetArgs([]string{
		"app",
		"development",
		"production",
		"--config", manifest,
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute(): %v", err)
	}

	output := out.String()
	if !strings.Contains(output, `"key": "HOST"`) {
		t.Errorf("output = %q, want JSON containing key HOST", output)
	}
}

func TestOutputDiffInvalidFormatFails(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	disabled := false
	console := printer.New(printer.Options{
		Out:   &buf,
		Err:   &buf,
		Color: &disabled,
	})

	err := outputDiff(console, &env.DiffResult{}, "invalid_format")
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
}
