package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
)

type mockEnvFactory struct {
	svc *env.Service
	err error
}

func (m *mockEnvFactory) EnvService(
	in *core.Input, project string,
) (*env.Service, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.svc != nil {
		return m.svc, nil
	}
	return core.NewApp().EnvService(in, project)
}

func writeGetManifest(t *testing.T) (manifestPath, dir string) {
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

	return manifestPath, dir
}

func writeYAML(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestOutputGetPrintsPlaintextWithNewline(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := outputGet(&buf, "dev-db.local")
	if err != nil {
		t.Fatalf("outputGet(): %v", err)
	}

	if got, want := buf.String(), "dev-db.local\n"; got != want {
		t.Errorf("outputGet() = %q, want %q", got, want)
	}
}

func TestNewGetCommand(t *testing.T) {
	t.Parallel()

	manifest, _ := writeGetManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	app := core.NewApp()

	svc, err := app.EnvService(input, "app")
	if err != nil {
		t.Fatalf("EnvService(): %v", err)
	}

	factory := &mockEnvFactory{svc: svc}
	cmd := NewGetCommand(factory)
	cmd.PersistentFlags().String("config", manifest, "")
	cmd.SetArgs([]string{
		"app",
		"host",
		"--config", manifest,
		"--env", "development",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute(): %v", err)
	}

	if got, want := out.String(), "dev-db.local\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNewGetCommandMissingKey(t *testing.T) {
	t.Parallel()

	manifest, _ := writeGetManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	app := core.NewApp()

	svc, err := app.EnvService(input, "app")
	if err != nil {
		t.Fatalf("EnvService(): %v", err)
	}

	factory := &mockEnvFactory{svc: svc}
	cmd := NewGetCommand(factory)
	cmd.PersistentFlags().String("config", manifest, "")
	cmd.SetArgs([]string{
		"app",
		"missing_key",
		"--config", manifest,
	})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing key")
	}
}
