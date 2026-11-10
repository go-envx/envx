//go:build e2e

package e2e_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	cfg := basicWorkspace(t)
	dist := filepath.Join(t.TempDir(), "dist")
	mustEnvx(t, nil, "pack", "--config", cfg, "-e", "development", "--out", dist)

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
	res := mustEnvx(t, nil, append([]string{
		"run", "--config", filepath.Join(dist, "envx.yaml"),
		"--env", "development", "--overload", "api-core", "--",
	}, helper(t, "printenv", "APP_NAME")...)...)
	if got := strings.TrimSpace(res.stdout); got != "api-core" {
		t.Errorf("child APP_NAME from bundle = %q, want api-core", got)
	}
}

// TestPackFiltersAndDecryptsSecrets is the end-to-end secret path: it packs a
// workspace with one encrypted, referenced secret and confirms the bundle store
// keeps only that secret (no public keys), fails closed without the private
// key, and decrypts at runtime with ENVX_PRIVATE_KEY.
func TestPackFiltersAndDecryptsSecrets(t *testing.T) {
	t.Parallel()

	cfg, privateKey := secretWorkspace(t)
	dist := filepath.Join(t.TempDir(), "dist")
	mustEnvx(t, nil, "pack", "--config", cfg, "-e", "production", "--out", dist)

	// The bundle store keeps only the referenced value and no public keys.
	//nolint:gosec // test-local path.
	store, err := os.ReadFile(filepath.Join(dist, "secrets.yaml"))
	if err != nil {
		t.Fatalf("bundle store missing: %v", err)
	}
	if !strings.Contains(string(store), "api_key:") {
		t.Errorf("bundle store missing referenced secret:\n%s", store)
	}
	if strings.Contains(string(store), "public-keys") {
		t.Errorf("bundle store still carries public keys:\n%s", store)
	}

	// Without the private key, the reference fails closed.
	bundleCfg := filepath.Join(dist, "envx.yaml")
	runArgs := append([]string{
		"run", "--config", bundleCfg, "--env", "production", "app", "--",
	}, helper(t, "printenv", "API_KEY")...)
	if res := envx(t, nil, runArgs...); res.code == 0 {
		t.Error("run from bundle succeeded without a private key")
	}

	// With ENVX_PRIVATE_KEY, the bundle decrypts at runtime.
	res := mustEnvx(t, []string{privateKey}, runArgs...)
	if got := strings.TrimSpace(res.stdout); got != "s3cr3t" {
		t.Errorf("decrypted API_KEY = %q, want s3cr3t", got)
	}
}

// TestEmitK8sSplitDecrypts is the end-to-end Kubernetes path: it emits the k8s
// target with each slice and confirms the split — "--only secrets" emits a
// Secret carrying the decrypted secret value (base64-encoded) while "--only
// config" emits a ConfigMap carrying only the plain value.
func TestEmitK8sSplitDecrypts(t *testing.T) {
	t.Parallel()

	cfg, privateKey := secretWorkspace(t)
	env := []string{privateKey}

	// The Secret carries the decrypted, base64-encoded secret and not the plain
	// value.
	secretOut := mustEnvx(t, env,
		"emit", "--config", cfg, "--env", "production", "app",
		"--target", "k8s", "--only", "secrets", "--name", "app-secrets",
	).stdout
	wantData := "API_KEY: " + base64.StdEncoding.EncodeToString([]byte("s3cr3t"))
	if !strings.Contains(secretOut, wantData) {
		t.Errorf("Secret missing decrypted secret %q:\n%s", wantData, secretOut)
	}
	if strings.Contains(secretOut, "GREETING") {
		t.Errorf("Secret unexpectedly carries the plain value:\n%s", secretOut)
	}

	// The ConfigMap carries the plain value and not the secret.
	configOut := mustEnvx(t, env,
		"emit", "--config", cfg, "--env", "production", "app",
		"--target", "k8s", "--only", "config", "--name", "app-config",
	).stdout
	if !strings.Contains(configOut, "GREETING: hello") {
		t.Errorf("ConfigMap missing the plain value:\n%s", configOut)
	}
	for _, secret := range []string{"API_KEY", "s3cr3t"} {
		if strings.Contains(configOut, secret) {
			t.Errorf("ConfigMap leaked secret material %q:\n%s", secret, configOut)
		}
	}
}

// TestEmitK8sBundleDecrypts is the end-to-end bundle path: it emits a merged
// --bundle and confirms the single Secret data key holds a base64 JSON body
// carrying both the decrypted secret and the plain value — the shape an app
// volume-mounts and parses as one file.
func TestEmitK8sBundleDecrypts(t *testing.T) {
	t.Parallel()

	cfg, privateKey := secretWorkspace(t)
	out := mustEnvx(t, []string{privateKey},
		"emit", "--config", cfg, "--env", "production", "app",
		"--target", "k8s-bundle", "--name", "app", "--key", "app.json",
	).stdout
	if !strings.Contains(out, "kind: Secret") {
		t.Fatalf("bundle should be a Secret:\n%s", out)
	}
	decoded, err := base64.StdEncoding.DecodeString(dataValue(t, out, "app.json"))
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

	cfg := basicWorkspace(t)
	mustEnvx(t, nil,
		"set", "--config", cfg, "--env", "development",
		"env/postgres", "feature.enabled", "true",
	)
	res := mustEnvx(t, nil,
		"get", "--config", cfg, "--env", "development",
		"api-core", "FEATURE_ENABLED",
	)
	if got := strings.TrimSpace(res.stdout); got != "true" {
		t.Errorf("FEATURE_ENABLED = %q, want true", got)
	}
}

// TestRunExitStatus verifies the child's exit status becomes envx's own, and a
// command that cannot start reports its cause on stderr with exit 127.
func TestRunExitStatus(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)

	t.Run("child status", func(t *testing.T) {
		t.Parallel()
		res := envx(t, nil, append(
			[]string{"run", "--config", cfg, "api-core", "--"},
			helper(t, "exit", "7")...,
		)...)
		if res.code != 7 {
			t.Errorf("exit %d, want 7\nstderr:\n%s", res.code, res.stderr)
		}
	})

	t.Run("command not found", func(t *testing.T) {
		t.Parallel()
		res := envx(t, nil, "run", "--config", cfg, "api-core", "--", "envx-no-such-cmd")
		if res.code != 127 {
			t.Errorf("exit %d, want 127", res.code)
		}
		if !strings.HasPrefix(res.stderr, "envx: ") ||
			!strings.Contains(res.stderr, "envx-no-such-cmd") {
			t.Errorf("stderr = %q, want envx-prefixed start failure", res.stderr)
		}
	})
}
