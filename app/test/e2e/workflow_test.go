//go:build e2e

package e2e_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
	"github.com/go-envx/envx/app/internal/utils/file"
)

// copyTree recursively copies the directory tree at src into dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyTree(t, s, d)
			continue
		}
		data, err := file.Read(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.WriteAtomic(d, data); err != nil {
			t.Fatal(err)
		}
	}
}

// dataValue extracts the scalar value of one data key from a rendered k8s
// manifest, so a test can decode a Secret's single bundle body.
func dataValue(t *testing.T, manifest, key string) string {
	t.Helper()
	for _, line := range strings.Split(manifest, "\n") {
		trimmed := strings.TrimSpace(line)
		if prefix := key + ": "; strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	t.Fatalf("data key %q not found in manifest:\n%s", key, manifest)
	return ""
}

// TestPackThenRun verifies pack produces a bundle that run executes through the
// ordinary --config pipeline: it copies the basic workspace scoped to one
// environment, then runs a command from the copied directory and confirms the
// merged environment reaches the child.
func TestPackThenRun(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	copyTree(t, fixtures.Testdata("basic"), work)
	cfg := filepath.Join(work, "envx.yaml")
	dist := filepath.Join(t.TempDir(), "dist")

	if _, _, err := execCmd(
		"pack", "--config", cfg, "-e", "development", "--out", dist,
	); err != nil {
		t.Fatalf("pack: %v", err)
	}

	// The bundle nests each namespace under its project directory, keeps the
	// selected environment's overlays, and drops the rest.
	kept := filepath.Join(dist, "api-core", "postgres.development.yaml")
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("selected development overlay missing from bundle: %v", err)
	}
	dropped := filepath.Join(dist, "api-core", "postgres.production.yaml")
	if _, err := os.Stat(dropped); !os.IsNotExist(err) {
		t.Error("unselected production overlay copied into bundle")
	}

	// The copied workspace runs through the ordinary pipeline under --config.
	stdout, _, err := execCmd(
		"run", "--config", filepath.Join(dist, "envx.yaml"),
		"--env", "development", "--overload",
		"api-core", "--", "printenv", "APP_NAME",
	)
	if err != nil {
		t.Fatalf("run from bundle: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "api-core" {
		t.Errorf("child APP_NAME from bundle = %q, want api-core", got)
	}
}

// TestPackFiltersAndDecryptsSecrets is the end-to-end secret path: it builds a
// workspace with a real keypair and one encrypted, referenced secret, packs it,
// and confirms the bundle store keeps only that secret (no public keys), fails
// closed without the private key, and decrypts at runtime with ENVX_PRIVATE_KEY.
func TestPackFiltersAndDecryptsSecrets(t *testing.T) {
	// Not parallel: t.Setenv drives the private-key lookup through the process env.
	work := t.TempDir()
	cfg := filepath.Join(work, "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: age\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(work, "env", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(appPath), 0o750); err != nil {
		t.Fatal(err)
	}
	appYAML := "GREETING: hello\nAPI_KEY: secret://app/api_key\n"
	if err := os.WriteFile(appPath, []byte(appYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	// Generate a keypair and store one encrypted secret.
	if _, _, err := execCmd("keypair", "generate", "app", "--config", cfg); err != nil {
		t.Fatalf("keypair generate: %v", err)
	}
	if _, _, err := execCmd(
		"secrets", "set", "app", "api_key", "s3cr3t", "--config", cfg,
	); err != nil {
		t.Fatalf("secrets set: %v", err)
	}
	//nolint:gosec // test-local path.
	privateKey, err := os.ReadFile(filepath.Join(work, "envx.keys"))
	if err != nil {
		t.Fatal(err)
	}

	// Pack the production environment.
	dist := filepath.Join(t.TempDir(), "dist")
	if _, _, err := execCmd(
		"pack", "--config", cfg, "-e", "production", "--out", dist,
	); err != nil {
		t.Fatalf("pack: %v", err)
	}

	// The bundle store keeps only the referenced value and no public keys.
	//nolint:gosec // test-local path.
	store, err := os.ReadFile(filepath.Join(dist, "secrets.yaml"))
	if err != nil {
		t.Fatalf("bundle store missing: %v", err)
	}
	if !strings.Contains(string(store), "api_key:") {
		t.Errorf("bundle store missing referenced secret:\n%s", store)
	}
	if strings.Contains(string(store), "public_keys") {
		t.Errorf("bundle store still carries public keys:\n%s", store)
	}

	// Without the private key, the reference fails closed.
	bundleCfg := filepath.Join(dist, "envx.yaml")
	if _, _, err := execCmd(
		"run", "--config", bundleCfg, "--env", "production",
		"app", "--", "printenv", "API_KEY",
	); err == nil {
		t.Error("run from bundle succeeded without a private key")
	}

	// With ENVX_PRIVATE_KEY, the bundle decrypts at runtime.
	t.Setenv("ENVX_PRIVATE_KEY", string(privateKey))
	stdout, _, err := execCmd(
		"run", "--config", bundleCfg, "--env", "production",
		"app", "--", "printenv", "API_KEY",
	)
	if err != nil {
		t.Fatalf("run from bundle with key: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "s3cr3t" {
		t.Errorf("decrypted API_KEY = %q, want s3cr3t", got)
	}
}

// TestEmitK8sSplitDecrypts is the end-to-end Kubernetes path: it builds a
// workspace with a real keypair and one encrypted, referenced secret, then emits
// the k8s target with each slice and confirms the split — "--only secrets" emits
// a Secret carrying the decrypted secret value (base64-encoded) while "--only
// config" emits a ConfigMap carrying only the plain value.
func TestEmitK8sSplitDecrypts(t *testing.T) {
	// Not parallel: t.Setenv drives the private-key lookup through the process env.
	work := t.TempDir()
	cfg := filepath.Join(work, "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: age\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(work, "env", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(appPath), 0o750); err != nil {
		t.Fatal(err)
	}
	appYAML := "GREETING: hello\nAPI_KEY: secret://app/api_key\n"
	if err := os.WriteFile(appPath, []byte(appYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	// Generate a keypair and store one encrypted secret.
	if _, _, err := execCmd("keypair", "generate", "app", "--config", cfg); err != nil {
		t.Fatalf("keypair generate: %v", err)
	}
	if _, _, err := execCmd(
		"secrets", "set", "app", "api_key", "s3cr3t", "--config", cfg,
	); err != nil {
		t.Fatalf("secrets set: %v", err)
	}
	//nolint:gosec // test-local path.
	privateKey, err := os.ReadFile(filepath.Join(work, "envx.keys"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENVX_PRIVATE_KEY", string(privateKey))

	// The Secret carries the decrypted, base64-encoded secret and not the plain
	// value.
	secretOut, _, err := execCmd(
		"emit", "--config", cfg, "--env", "production", "app",
		"--target", "k8s", "--only", "secrets", "--name", "app-secrets",
	)
	if err != nil {
		t.Fatalf("emit k8s --only secrets: %v", err)
	}
	wantData := "API_KEY: " + base64.StdEncoding.EncodeToString([]byte("s3cr3t"))
	if !strings.Contains(secretOut.String(), wantData) {
		t.Errorf(
			"Secret missing decrypted secret %q:\n%s",
			wantData,
			secretOut.String(),
		)
	}
	if strings.Contains(secretOut.String(), "GREETING") {
		t.Errorf("Secret unexpectedly carries the plain value:\n%s", secretOut.String())
	}

	// The ConfigMap carries the plain value and not the secret.
	configOut, _, err := execCmd(
		"emit", "--config", cfg, "--env", "production", "app",
		"--target", "k8s", "--only", "config", "--name", "app-config",
	)
	if err != nil {
		t.Fatalf("emit k8s --only config: %v", err)
	}
	if !strings.Contains(configOut.String(), "GREETING: hello") {
		t.Errorf("ConfigMap missing the plain value:\n%s", configOut.String())
	}
	for _, secret := range []string{"API_KEY", "s3cr3t"} {
		if strings.Contains(configOut.String(), secret) {
			t.Errorf(
				"ConfigMap leaked secret material %q:\n%s",
				secret,
				configOut.String(),
			)
		}
	}
}

// TestEmitK8sBundleDecrypts is the end-to-end bundle path: it reuses the
// encrypted workspace and emits a merged --bundle, then confirms the single
// Secret data key holds a base64 JSON body carrying both the decrypted secret and
// the plain value — the shape an app volume-mounts and parses as one file.
func TestEmitK8sBundleDecrypts(t *testing.T) {
	// Not parallel: t.Setenv drives the private-key lookup through the process env.
	work := t.TempDir()
	cfg := filepath.Join(work, "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: age\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(work, "env", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(appPath), 0o750); err != nil {
		t.Fatal(err)
	}
	appYAML := "GREETING: hello\nAPI_KEY: secret://app/api_key\n"
	if err := os.WriteFile(appPath, []byte(appYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execCmd("keypair", "generate", "app", "--config", cfg); err != nil {
		t.Fatalf("keypair generate: %v", err)
	}
	if _, _, err := execCmd(
		"secrets", "set", "app", "api_key", "s3cr3t", "--config", cfg,
	); err != nil {
		t.Fatalf("secrets set: %v", err)
	}
	//nolint:gosec // test-local path.
	privateKey, err := os.ReadFile(filepath.Join(work, "envx.keys"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENVX_PRIVATE_KEY", string(privateKey))

	out, _, err := execCmd(
		"emit", "--config", cfg, "--env", "production", "app",
		"--target", "k8s-bundle", "--name", "app", "--key", "app.json",
	)
	if err != nil {
		t.Fatalf("emit k8s-bundle: %v", err)
	}
	// The merged bundle is a single Secret data key; decode its base64 body and
	// confirm it is JSON carrying both the decrypted secret and the plain value.
	if !strings.Contains(out.String(), "kind: Secret") {
		t.Fatalf("bundle should be a Secret:\n%s", out.String())
	}
	encoded := dataValue(t, out.String(), "app.json")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("bundle body is not valid base64: %v", err)
	}
	for _, want := range []string{`"API_KEY": "s3cr3t"`, `"GREETING": "hello"`} {
		if !strings.Contains(string(decoded), want) {
			t.Errorf("bundle body missing %q:\n%s", want, decoded)
		}
	}
}

// TestSetRoundTrip verifies set writes a value that get can read back.
func TestSetRoundTrip(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	copyTree(t, fixtures.Testdata("basic"), work)
	cfg := filepath.Join(work, "envx.yaml")

	if _, _, err := execCmd(
		"set", "--config", cfg, "--env", "development",
		"env/postgres", "feature.enabled", "true",
	); err != nil {
		t.Fatalf("set: %v", err)
	}

	stdout, _, err := execCmd(
		"get", "--config", cfg, "--env", "development",
		"api-core", "FEATURE_ENABLED",
	)
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "true" {
		t.Errorf("FEATURE_ENABLED = %q, want true", got)
	}
}
