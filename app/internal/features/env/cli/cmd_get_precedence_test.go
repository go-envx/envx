package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writePrecedenceManifest creates a workspace whose project setting selects the
// development environment, with a development overlay that changes host.
func writePrecedenceManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	manifest := filepath.Join(dir, "envx.yaml")
	body := "environments: [development, production]\n" +
		"projects:\n" +
		"  app:\n" +
		"    includes: [env/postgres]\n" +
		"    settings:\n" +
		"      env: development\n"
	if err := os.WriteFile(manifest, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	envDir := filepath.Join(dir, "env")
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, envDir, "postgres.yaml", "host: localhost\n")
	writeYAML(t, envDir, "postgres.development.yaml", "host: dev-db.local\n")
	return manifest
}

// TestGetEnvironmentPrecedence pins the order flag > ENVX_ENV > project setting.
// It cannot run in parallel because it sets process environment variables.
func TestGetEnvironmentPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		args   []string
		want   string
	}{
		{name: "project setting by default", want: "dev-db.local\n"},
		{name: "ENVX_ENV over project setting", envVar: "production", want: "localhost\n"},
		{
			name:   "flag over ENVX_ENV",
			envVar: "production",
			args:   []string{"--env", "development"},
			want:   "dev-db.local\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ENVX_ENV", test.envVar)

			manifest := writePrecedenceManifest(t)
			cmd := NewGetCommand(&mockEnvFactory{})
			cmd.PersistentFlags().String("config", manifest, "")
			cmd.SetArgs(append(
				[]string{"app", "host", "--config", manifest}, test.args...,
			))

			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("cmd.Execute(): %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("output = %q, want %q", got, test.want)
			}
		})
	}
}
