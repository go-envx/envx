package app

import (
	"github.com/go-envx/envx/app/internal/core/env"
	envStore "github.com/go-envx/envx/app/internal/core/env/filestore"
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
}

// newValueResolver builds a resolver whose keyring reads private-key overrides
// from the operation's host snapshot.
func (f valueResolverFactory) newValueResolver(
	reveal bool, host hostenv.Snapshot,
) (env.ValueResolver, error) {
	keys, err := keyring.New(f.keysPath, host.Get)
	if err != nil {
		return nil, err
	}
	resolver, err := secrets.NewResolver(f.secretsReader, keys, f.cipher, reveal)
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
	}
	return env.New(*loadedWorkspace, env.Dependencies{
		EnvNamespaces:    envStore.Repository{},
		EnvOverlays:      envStore.Repository{},
		HostEnv:          hostEnv,
		NewValueResolver: factory.newValueResolver,
		ProcessRunner:    process.Run,
	})
}
