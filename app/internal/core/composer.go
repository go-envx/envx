package core

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/env"
	envfilestore "github.com/go-envx/envx/app/internal/features/env/filestore"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/privatekey"
	pkfilestore "github.com/go-envx/envx/app/internal/features/privatekey/filestore"
	"github.com/go-envx/envx/app/internal/features/secrets"
	secfilestore "github.com/go-envx/envx/app/internal/features/secrets/filestore"
	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/resources/procenv"
)

// newConfiguredSecretsService composes the secrets service for a secrets config.
func newConfiguredSecretsService(config secrets.Config) (*secrets.Service, error) {
	return NewSecretsService(
		config.SecretsPath,
		config.KeysPath,
		cipher.Params{Algorithm: cipher.Algorithm(config.Cipher)},
		config.DefaultIndent,
	)
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

	procEnv, err := procenv.New(procenv.Params{})
	if err != nil {
		return nil, fmt.Errorf("creating process environment client: %w", err)
	}

	pkService, err := privatekey.NewService(privatekey.ServiceParams{
		Repository: pkRepo,
		LookupEnv:  procEnv.LookupEnv,
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
		Cipher:            oCipher,
		PrivateKeyService: pkService,
	})
}

// resolverFactory opens a fresh secrets service and operation-scoped resolver for
// each resolving env operation, so no store snapshot or private-key cache
// survives it. It implements env.ValueResolverFactory.
type resolverFactory struct {
	secrets secrets.Config
}

// Resolver composes a fresh secrets service and opens a resolver under the
// reveal policy.
func (f resolverFactory) Resolver(reveal bool) (env.ValueResolver, error) {
	manager, err := newConfiguredSecretsService(f.secrets)
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
	repo, err := envfilestore.New(envfilestore.Params{})
	if err != nil {
		return nil, fmt.Errorf("creating namespace repository: %w", err)
	}

	procEnv, err := procenv.New(procenv.Params{})
	if err != nil {
		return nil, fmt.Errorf("creating process environment client: %w", err)
	}

	return env.NewService(env.ServiceParams{
		Config:          res.env,
		Repository:      repo,
		ResolverFactory: resolverFactory{secrets: res.secrets},
		OSEnvironment:   procEnv.Environ(),
	})
}

// newPackService composes a pack.Service over a pack config, reading the
// workspace secrets store and writing bundled stores through secrets filestores.
func newPackService(config pack.Config) (*pack.Service, error) {
	secStore, err := secfilestore.New(secfilestore.Params{Path: config.SecretsPath})
	if err != nil {
		return nil, fmt.Errorf("creating secrets store: %w", err)
	}

	return pack.NewService(pack.ServiceParams{
		Config:        config,
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
	envService, err := newWorkspaceEnvService(res)
	if err != nil {
		return nil, err
	}

	secretsService, err := newConfiguredSecretsService(res.secrets)
	if err != nil {
		return nil, err
	}

	return validate.NewService(validate.ServiceParams{
		Config:         res.validate,
		EnvService:     envService,
		SecretsService: secretsService,
	})
}
