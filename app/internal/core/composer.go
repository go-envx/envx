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
	"github.com/go-envx/envx/app/internal/features/workspace"
	wsfilestore "github.com/go-envx/envx/app/internal/features/workspace/filestore"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// NewConfiguredCipher resolves the workspace cipher when a manifest is present,
// or constructs the application's default cipher without a workspace.
func NewConfiguredCipher(in *Input) (cipher.Cipher, error) {
	// Bind the resolved manifest path and conventional filename into a repository.
	wsRepo, err := wsfilestore.New(wsfilestore.Params{
		Path:     resolveManifestPath(in),
		Filename: defaultManifestFilename,
	})
	if err != nil {
		return nil, err
	}

	wsService, err := workspace.NewService(workspace.ServiceParams{
		Repository: wsRepo,
	})
	if err != nil {
		return nil, err
	}

	// Replace the default with the manifest's algorithm when a workspace exists.
	cipherParams := cipher.Params{Algorithm: defaultCipherAlgorithm}
	ws, err := wsService.Load()
	if err != nil {
		if !errors.Is(err, workspace.ErrNotFound) {
			return nil, err
		}
	} else {
		cipherParams = resolveCipherParams(manifestContext{
			workspace: ws,
		})
	}

	// Construct the selected implementation and add context if construction fails.
	selectedCipher, err := cipher.New(cipherParams)
	if err != nil {
		return nil, fmt.Errorf("creating configured cipher: %w", err)
	}
	return selectedCipher, nil
}

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

// NewSecretsManager composes the configured cipher and private-key ports into
// a secrets service for one resolved workspace.
func NewSecretsManager(s SecretsParams, c cipher.Params) (*secrets.Service, error) {
	return NewSecretsService(s.SecretsPath, s.KeysPath, c, s.DefaultIndent)
}

// NewEnvService constructs an env.Service with a local filestore repository.
//
//nolint:gocritic // constructor parameter matches domain convention.
func NewEnvService(params env.ServiceParams) (*env.Service, error) {
	if params.Repository == nil {
		params.Repository = envfilestore.New(envfilestore.Params{})
	}
	return env.NewService(params)
}

// NewWorkspaceEnvService composes a workspace-level env.Service from a resolved
// workspace.
func NewWorkspaceEnvService(res *Result) (*env.Service, error) {
	if res == nil || res.workspace == nil {
		return nil, errors.New("workspace is required")
	}

	projects := make(map[string]env.ProjectConfig, len(res.workspace.Projects))
	for name, p := range res.workspace.Projects {
		absIncludes := make([]string, len(p.Includes))
		for i, inc := range p.Includes {
			absIncludes[i] = filepath.Join(res.dir, inc)
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
		secrets: res.Secrets,
		cipher:  res.Cipher,
	}

	defaultEnv := res.workspace.DefaultEnvironment()
	if res.workspace.Settings.Env != nil && *res.workspace.Settings.Env != "" {
		defaultEnv = *res.workspace.Settings.Env
	}

	globalSettings := env.Settings{
		Delimiter: env.PrecedenceString(
			&env.Delimiter, res.workspace.Settings.Delimiter,
		),
		Prefix: env.PrecedenceString(
			&env.Prefix, res.workspace.Settings.Prefix,
		),
		Suffix: env.PrecedenceString(
			&env.Suffix, res.workspace.Settings.Suffix,
		),
		ReferencePattern: env.PrecedenceString(
			&env.ReferencePattern, res.workspace.Settings.ReferencePattern,
		),
		RequireOverlays: env.PrecedenceBool(
			&env.RequireOverlays, res.workspace.Settings.RequireOverlays,
		),
		Overload: env.PrecedenceBool(
			&env.Overload, res.workspace.Settings.Overload,
		),
	}

	return env.NewService(env.ServiceParams{
		WorkspaceDir:       res.dir,
		Repository:         repo,
		Projects:           projects,
		Environments:       res.workspace.Environments,
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

// NewValidateService composes a validate.Service from a resolved workspace: the
// workspace-level env.Service diagnoses each project environment and the secrets
// service supplies the store-level findings.
func NewValidateService(res *Result) (*validate.Service, error) {
	if res == nil || res.workspace == nil {
		return nil, errors.New("workspace is required")
	}

	envService, err := NewWorkspaceEnvService(res)
	if err != nil {
		return nil, err
	}

	secretsService, err := NewSecretsManager(res.Secrets, res.Cipher)
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
