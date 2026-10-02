package core

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/test/fixtures"
)

// TestResolveWorkspace verifies the resolved secrets store, key file, and cipher
// defaults for a fixture manifest.
func TestResolveWorkspace(t *testing.T) {
	t.Parallel()

	res, err := resolveWorkspace(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("resolveWorkspace: %v", err)
	}
	if filepath.Base(res.secrets.SecretsPath) != "secrets.yaml" {
		t.Errorf("SecretsPath = %q, want .../secrets.yaml", res.secrets.SecretsPath)
	}
	wantKeys := filepath.Join(filepath.Dir(res.secrets.SecretsPath), "envx.keys")
	if res.secrets.KeysPath != wantKeys {
		t.Errorf("KeysPath = %q, want %q", res.secrets.KeysPath, wantKeys)
	}
	if res.cipher.Algorithm != defaultCipherAlgorithm {
		t.Errorf("Algorithm = %q, want %q", res.cipher.Algorithm, defaultCipherAlgorithm)
	}
	if res.cipher.Options != nil {
		t.Errorf("Cipher.Options = %T, want nil defaults", res.cipher.Options)
	}
}

// TestResolveSecretsParams verifies store, key, and indent resolution against an
// in-memory workspace.
func TestResolveSecretsParams(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	absoluteKeys := filepath.Join(t.TempDir(), "envx.keys")

	tests := []struct {
		name    string
		secrets workspace.SecretsConfig
		indent  int
		want    SecretsParams
	}{
		{
			name:   "defaults beside the manifest",
			indent: 2,
			want: SecretsParams{
				SecretsPath:   filepath.Join(dir, "secrets.yaml"),
				KeysPath:      filepath.Join(dir, "envx.keys"),
				DefaultIndent: 2,
			},
		},
		{
			name:    "keys default beside a custom store",
			secrets: workspace.SecretsConfig{SecretsPath: "private/secrets.yaml"},
			indent:  2,
			want: SecretsParams{
				SecretsPath:   filepath.Join(dir, "private", "secrets.yaml"),
				KeysPath:      filepath.Join(dir, "private", "envx.keys"),
				DefaultIndent: 2,
			},
		},
		{
			name: "relative keys resolve against the manifest",
			secrets: workspace.SecretsConfig{
				SecretsPath: "private/secrets.yaml",
				KeysPath:    "keys/envx.keys",
			},
			indent: 2,
			want: SecretsParams{
				SecretsPath:   filepath.Join(dir, "private", "secrets.yaml"),
				KeysPath:      filepath.Join(dir, "keys", "envx.keys"),
				DefaultIndent: 2,
			},
		},
		{
			name:    "absolute keys stay rooted",
			secrets: workspace.SecretsConfig{KeysPath: absoluteKeys},
			indent:  2,
			want: SecretsParams{
				SecretsPath:   filepath.Join(dir, "secrets.yaml"),
				KeysPath:      absoluteKeys,
				DefaultIndent: 2,
			},
		},
		{
			name:   "detected indent flows through",
			indent: 4,
			want: SecretsParams{
				SecretsPath:   filepath.Join(dir, "secrets.yaml"),
				KeysPath:      filepath.Join(dir, "envx.keys"),
				DefaultIndent: 4,
			},
		},
		{
			name:   "out-of-range indent falls back",
			indent: 12,
			want: SecretsParams{
				SecretsPath:   filepath.Join(dir, "secrets.yaml"),
				KeysPath:      filepath.Join(dir, "envx.keys"),
				DefaultIndent: defaultIndent,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ws := &workspace.Workspace{
				Root:    dir,
				Indent:  test.indent,
				Secrets: test.secrets,
			}
			if got := resolveSecretsParams(ws); got != test.want {
				t.Errorf("resolveSecretsParams() = %+v, want %+v", got, test.want)
			}
		})
	}
}

// TestResolveCipherParams verifies the manifest cipher overrides the default.
func TestResolveCipherParams(t *testing.T) {
	t.Parallel()

	ws := &workspace.Workspace{}
	if got := resolveCipherParams(ws).Algorithm; got != defaultCipherAlgorithm {
		t.Errorf("Algorithm = %q, want default %q", got, defaultCipherAlgorithm)
	}
	ws.Secrets.Cipher = string(cipher.NaClBox)
	if got := resolveCipherParams(ws).Algorithm; got != cipher.NaClBox {
		t.Errorf("Algorithm = %q, want %q", got, cipher.NaClBox)
	}
}

// TestEnvServiceResolvesFixtureProject verifies the composed env service
// resolves a known fixture project end to end.
func TestEnvServiceResolvesFixtureProject(t *testing.T) {
	t.Parallel()

	service, err := NewApp().EnvService(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
	entry, err := service.Get(env.GetParams{Project: "api-core", Key: "APP_NAME"})
	if err != nil {
		t.Fatalf("Get APP_NAME: %v", err)
	}
	if entry.Value != "api-core" {
		t.Errorf("APP_NAME = %q, want api-core", entry.Value)
	}
}

// TestEnvServiceMasksSecretReference verifies read commands mask secret
// references by default without decrypting or requiring a private key.
func TestEnvServiceMasksSecretReference(t *testing.T) {
	t.Parallel()

	service, err := NewApp().EnvService(fixtures.Manifest("resolve/secret-reference"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
	password, err := service.Get(env.GetParams{Project: "api", Key: "PASSWORD"})
	if err != nil {
		t.Fatalf("Get PASSWORD: %v", err)
	}
	if password.Value != "secret://development/api_key" {
		t.Errorf("PASSWORD = %q, want masked development reference", password.Value)
	}
	token, err := service.Get(env.GetParams{Project: "api", Key: "TOKEN"})
	if err != nil {
		t.Fatalf("Get TOKEN: %v", err)
	}
	if token.Value != "secret://shared/token" {
		t.Errorf("TOKEN = %q, want masked shared reference", token.Value)
	}
}

// TestEnvServiceIgnoresSecretsStoreAtConstruction verifies composing the env
// service never reads a malformed secrets store.
func TestEnvServiceIgnoresSecretsStoreAtConstruction(t *testing.T) {
	t.Parallel()

	_, err := NewApp().EnvService(fixtures.Manifest("resolve/global-ignores-store"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
}

// TestEnvServiceDanglingSecretReference verifies a reference with no matching
// store entry masks to its canonical form for a default read but fails loudly
// once materialized.
func TestEnvServiceDanglingSecretReference(t *testing.T) {
	t.Parallel()

	service, err := NewApp().EnvService(fixtures.Manifest("resolve/dangling-reference"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
	entry, err := service.Get(env.GetParams{Project: "api", Key: "PASSWORD"})
	if err != nil {
		t.Fatalf("Get masked: %v", err)
	}
	if entry.Value != "secret://development/missing" {
		t.Errorf("PASSWORD = %q, want the masked reference", entry.Value)
	}
	if _, err := service.Materialize(env.MaterializeParams{Project: "api"}); err == nil {
		t.Fatal("expected dangling reference error when materialized")
	}
}
