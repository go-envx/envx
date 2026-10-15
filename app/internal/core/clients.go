package core

import (
	"fmt"

	envfilestore "github.com/go-envx/envx/app/internal/features/env/filestore"
	pkfilestore "github.com/go-envx/envx/app/internal/features/privatekey/filestore"
	secfilestore "github.com/go-envx/envx/app/internal/features/secrets/filestore"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/resources/cipher/age"
	"github.com/go-envx/envx/app/internal/resources/cipher/naclbox"
	"github.com/go-envx/envx/app/internal/resources/hostenv"
)

// AppClients holds every client the workspace-bound services consume.
type AppClients struct {
	Cipher          *cipher.Client
	HostEnv         *hostenv.Client
	PrivateKeyStore *pkfilestore.Repository
	SecretsStore    *secfilestore.Repository
	SecretsExporter *secfilestore.Exporter
	NamespaceStore  *envfilestore.Repository
}

// composeAppClients constructs each client from its config slice.
func composeAppClients(config *AppConfig) (AppClients, error) {
	ageProvider, err := age.New(age.Params{})
	if err != nil {
		return AppClients{}, fmt.Errorf("composing age cipher: %w", err)
	}

	naclProvider, err := naclbox.New(naclbox.Params{})
	if err != nil {
		return AppClients{}, fmt.Errorf("composing nacl-box cipher: %w", err)
	}

	cipherClient, err := cipher.New(cipher.Params{
		Providers: []cipher.Provider{ageProvider, naclProvider},
	})
	if err != nil {
		return AppClients{}, fmt.Errorf("composing cipher: %w", err)
	}

	hostEnv := hostenv.New()

	keysPath := config.Secrets.KeysPath
	privateKeyStore, err := pkfilestore.New(pkfilestore.Params{Path: keysPath})
	if err != nil {
		return AppClients{}, fmt.Errorf("composing private key store: %w", err)
	}

	secretsStore, err := secfilestore.New(secfilestore.Params{
		Path:          config.Secrets.SecretsPath,
		DefaultIndent: config.Secrets.DefaultIndent,
	})
	if err != nil {
		return AppClients{}, fmt.Errorf("composing secrets store: %w", err)
	}

	secretsExporter, err := secfilestore.NewExporter(secfilestore.ExporterParams{
		DefaultIndent: config.Secrets.DefaultIndent,
	})
	if err != nil {
		return AppClients{}, fmt.Errorf("composing secrets exporter: %w", err)
	}

	namespaceStore, err := envfilestore.New(envfilestore.Params{})
	if err != nil {
		return AppClients{}, fmt.Errorf("composing namespace store: %w", err)
	}

	return AppClients{
		Cipher:          cipherClient,
		HostEnv:         hostEnv,
		PrivateKeyStore: privateKeyStore,
		SecretsStore:    secretsStore,
		SecretsExporter: secretsExporter,
		NamespaceStore:  namespaceStore,
	}, nil
}
