package app

import (
	"github.com/go-envx/envx/app/internal/core/env"
	envStore "github.com/go-envx/envx/app/internal/core/env/filestore"
	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/core/external/onepassword"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/secrets/keyring"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
)

// valueResolverFactory builds operation-scoped value resolvers.
type valueResolverFactory struct {
	// secretsReader reads the workspace secrets store.
	secretsReader secrets.Reader
	// keysPath locates the private keyring file.
	keysPath string
	// cipher decrypts stored values.
	cipher secrets.Decrypter
	// providers configures the declared external providers by scheme.
	providers map[string]workspace.ProviderConfig
}

// newValueResolver builds a resolver whose keyring and external providers read
// overrides and credentials from the operation's host snapshot.
func (f valueResolverFactory) newValueResolver(
	params env.ResolverParams,
) (env.ValueResolver, error) {
	keys, err := keyring.New(f.keysPath, params.Host.Get)
	if err != nil {
		return nil, err
	}
	storeResolver, err := secrets.NewResolver(
		f.secretsReader, keys, f.cipher, params.Reveal,
	)
	if err != nil {
		return nil, err
	}
	resolver, err := external.NewResolver(external.ResolverParams{
		Next:      storeResolver,
		Reveal:    params.Reveal,
		Providers: newProviders(f.providers, params.Host),
	})
	if err != nil {
		return nil, err
	}
	return resolver, nil
}

// NewEnvService assembles the environment service for the workspace at configPath.
func (f Factory) NewEnvService(configPath string) (*env.Service, error) {
	loadedWorkspace, err := loadWorkspace(configPath)
	if err != nil {
		return nil, err
	}
	return envService(&loadedWorkspace, f.hostEnvironment())
}

// envService wires the environment service; each operation's resolver reads
// private-key overrides from that operation's host snapshot.
func envService(
	loadedWorkspace *workspace.Workspace,
	hostEnv func() hostenv.Snapshot,
) (*env.Service, error) {
	cipher, err := newCipher(loadedWorkspace.Secrets.Cipher)
	if err != nil {
		return nil, err
	}
	store, err := newSecretsStore(loadedWorkspace)
	if err != nil {
		return nil, err
	}
	factory := valueResolverFactory{
		secretsReader: store,
		keysPath:      loadedWorkspace.Secrets.KeysPath,
		cipher:        cipher,
		providers:     loadedWorkspace.Secrets.Providers,
	}
	return env.New(*loadedWorkspace, env.Dependencies{
		EnvNamespaces:    envStore.Repository{},
		EnvOverlays:      envStore.Repository{},
		HostEnv:          hostEnv,
		NewValueResolver: factory.newValueResolver,
		ProcessRunner:    process.Run,
	})
}

// newProviders selects the adapter for each declared provider. Providers
// without an adapter stay unregistered, so their references pass through.
func newProviders(
	configs map[string]workspace.ProviderConfig, host hostenv.Snapshot,
) []external.Registration {
	var registrations []external.Registration
	for scheme, config := range configs {
		var provider external.Provider
		switch scheme {
		case external.SchemeOnePassword:
			provider = onepassword.New(process.Output, host)
		default:
			continue
		}
		registrations = append(registrations, external.Registration{
			Provider: provider,
			Timeout:  config.Timeout,
		})
	}
	return registrations
}
