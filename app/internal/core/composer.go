package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-envx/envx/app/internal/features/env"
	envfilestore "github.com/go-envx/envx/app/internal/features/env/filestore"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/privatekey"
	pkfilestore "github.com/go-envx/envx/app/internal/features/privatekey/filestore"
	"github.com/go-envx/envx/app/internal/features/secrets"
	secfilestore "github.com/go-envx/envx/app/internal/features/secrets/filestore"
	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// cipherAdapter adapts a cipher.Cipher to the secrets.CipherClient interface.
type cipherAdapter struct {
	cipher cipher.Cipher
}

func (a cipherAdapter) Algorithm() string {
	return string(a.cipher.Algorithm())
}

func (a cipherAdapter) Keypair() (publicKey, privateKey string, err error) {
	kp, err := a.cipher.Keypair()
	if err != nil {
		return "", "", err
	}
	return kp.PublicKey, kp.PrivateKey, nil
}

func (a cipherAdapter) ValidateKeypair(publicKey, privateKey string) error {
	return a.cipher.ValidateKeypair(publicKey, privateKey)
}

func (a cipherAdapter) Encrypt(plaintext, publicKey string) ([]byte, error) {
	return a.cipher.Encrypt(plaintext, publicKey)
}

func (a cipherAdapter) Decrypt(ciphertext []byte, privateKey string) (string, error) {
	return a.cipher.Decrypt(ciphertext, privateKey)
}

// SecretsParams supplies paths and configuration for secrets composition.
type SecretsParams struct {
	SecretsPath   string
	KeysPath      string
	DefaultIndent int
}

// NewSecretsService composes the configured cipher, filestores, and domain services.
func NewSecretsService(
	secretsPath, keysPath string,
	cipherParams cipher.Params,
	indent int,
) (*secrets.Service, error) {
	oCipher, err := cipher.New(cipherParams)
	if err != nil {
		return nil, fmt.Errorf("creating configured cipher: %w", err)
	}

	pkRepo, err := pkfilestore.New(pkfilestore.Params{Path: keysPath})
	if err != nil {
		return nil, fmt.Errorf("creating privatekey repository: %w", err)
	}

	pkService, err := privatekey.NewService(privatekey.ServiceParams{
		Repository: pkRepo,
		LookupEnv:  os.LookupEnv,
	})
	if err != nil {
		return nil, fmt.Errorf("creating privatekey service: %w", err)
	}

	secStore, err := secfilestore.New(secfilestore.Params{
		Path:          secretsPath,
		DefaultIndent: indent,
	})
	if err != nil {
		return nil, fmt.Errorf("creating secrets store: %w", err)
	}

	return secrets.NewService(secrets.ServiceParams{
		Repository:        secStore,
		Cipher:            cipherAdapter{cipher: oCipher},
		PrivateKeyService: pkService,
	})
}

// resolverFactory opens a fresh secrets service and operation-scoped resolver for
// each resolving env operation, so no store snapshot or private-key cache
// survives it. It implements env.ValueResolverFactory.
type resolverFactory struct {
	secrets SecretsParams
	cipher  cipher.Params
}

// Resolver composes a fresh secrets service and opens a resolver under the
// reveal policy.
func (f resolverFactory) Resolver(reveal bool) (env.ValueResolver, error) {
	manager, err := NewSecretsService(
		f.secrets.SecretsPath, f.secrets.KeysPath, f.cipher, f.secrets.DefaultIndent,
	)
	if err != nil {
		return nil, err
	}
	resolver, err := manager.Resolver(secrets.ResolverParams{Reveal: reveal})
	if err != nil {
		return nil, err
	}
	return resolver, nil
}

// newWorkspaceEnvService composes a workspace-level env.Service from a resolved
// workspace.
func newWorkspaceEnvService(res *resolvedWorkspace) (*env.Service, error) {
	if res == nil || res.workspace == nil {
		return nil, errors.New("workspace is required")
	}
	ws := res.workspace

	projects := make(map[string]env.ProjectConfig, len(ws.Projects))
	for name, p := range ws.Projects {
		absIncludes := make([]string, len(p.Includes))
		for i, inc := range p.Includes {
			absIncludes[i] = filepath.Join(ws.Root, inc)
		}
		projects[name] = env.ProjectConfig{
			Name:     name,
			Includes: absIncludes,
			Settings: env.Options{
				Delimiter:        p.Settings.Delimiter,
				Env:              p.Settings.Env,
				Overload:         p.Settings.Overload,
				Prefix:           p.Settings.Prefix,
				ReferencePattern: p.Settings.ReferencePattern,
				RequireOverlays:  p.Settings.RequireOverlays,
				Suffix:           p.Settings.Suffix,
			},
		}
	}

	repo := envfilestore.New(envfilestore.Params{})

	resolverFact := resolverFactory{
		secrets: res.secrets,
		cipher:  res.cipher,
	}

	defaultEnv := ws.DefaultEnvironment()
	if ws.Settings.Env != nil && *ws.Settings.Env != "" {
		defaultEnv = *ws.Settings.Env
	}

	globalSettings := env.Settings{
		Delimiter:        env.PrecedenceString(ws.Settings.Delimiter),
		Prefix:           env.PrecedenceString(ws.Settings.Prefix),
		Suffix:           env.PrecedenceString(ws.Settings.Suffix),
		ReferencePattern: env.PrecedenceString(ws.Settings.ReferencePattern),
		RequireOverlays:  env.PrecedenceBool(ws.Settings.RequireOverlays),
		Overload:         env.PrecedenceBool(ws.Settings.Overload),
	}

	return env.NewService(env.ServiceParams{
		WorkspaceDir:       ws.Root,
		Repository:         repo,
		Projects:           projects,
		Environments:       ws.Environments,
		DefaultEnvironment: defaultEnv,
		Settings:           globalSettings,
		ResolverFactory:    resolverFact,
		OSEnvironment:      osEnvironment(),
	})
}

// NewPackService composes a pack.Service over a resolved workspace layout,
// reading the workspace secrets store and writing bundled stores through secrets
// filestores.
func NewPackService(layout *WorkspaceLayout) (*pack.Service, error) {
	if layout == nil {
		return nil, errors.New("workspace layout is required")
	}

	projects := make([]pack.Project, 0, len(layout.Projects))
	for _, project := range layout.Projects {
		projects = append(projects, pack.Project{
			Name:     project.Name,
			Includes: project.Includes,
		})
	}

	secStore, err := secfilestore.New(secfilestore.Params{Path: layout.SecretsPath})
	if err != nil {
		return nil, fmt.Errorf("creating secrets store: %w", err)
	}

	return pack.NewService(pack.ServiceParams{
		Workspace: pack.Workspace{
			ManifestPath: layout.ManifestPath,
			Root:         layout.Root,
			SecretsPath:  layout.SecretsPath,
			Environments: layout.Environments,
			Projects:     projects,
		},
		SecretsReader: secStore,
		NewSecretsWriter: func(path string) (pack.SecretsWriter, error) {
			bundleStore, err := secfilestore.New(secfilestore.Params{Path: path})
			if err != nil {
				return nil, err
			}
			return bundleStore, nil
		},
	})
}

// newValidateService composes a validate.Service from a resolved workspace: the
// workspace-level env.Service diagnoses each project environment and the secrets
// service supplies the store-level findings.
func newValidateService(res *resolvedWorkspace) (*validate.Service, error) {
	if res == nil || res.workspace == nil {
		return nil, errors.New("workspace is required")
	}

	envService, err := newWorkspaceEnvService(res)
	if err != nil {
		return nil, err
	}

	secretsService, err := NewSecretsService(
		res.secrets.SecretsPath, res.secrets.KeysPath, res.cipher,
		res.secrets.DefaultIndent,
	)
	if err != nil {
		return nil, err
	}

	// The manifest already validated the validate block at load, so this only
	// re-keys it by canonical code.
	severity, err := status.Resolve(res.workspace.ValidateSeverities)
	if err != nil {
		return nil, err
	}

	// Diagnose projects in sorted name order for deterministic output.
	projects := make([]string, 0, len(res.workspace.Projects))
	for name := range res.workspace.Projects {
		projects = append(projects, name)
	}
	sort.Strings(projects)

	return validate.NewService(validate.ServiceParams{
		Environment:  envService,
		Store:        secretsService,
		Projects:     projects,
		Environments: res.workspace.Environments,
		Severity:     severity,
	})
}
