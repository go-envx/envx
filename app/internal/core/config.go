package core

import (
	"path/filepath"

	"github.com/go-envx/envx/app/internal/features/workspace"
	wsfilestore "github.com/go-envx/envx/app/internal/features/workspace/filestore"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/utils/filex"
)

const (
	// defaultManifestFilename is the conventional workspace manifest filename.
	defaultManifestFilename = "envx.yaml"
	// defaultSecretsFilename is the default workspace secrets store filename.
	defaultSecretsFilename = "secrets.yaml"
	// defaultKeysFilename is the default workspace private-key filename.
	defaultKeysFilename = "envx.keys"
	// defaultCipherAlgorithm is the application's default encryption algorithm.
	defaultCipherAlgorithm = cipher.Age
	// defaultIndent is the block indentation used for yaml files.
	defaultIndent = 2
)

// resolvedWorkspace is a loaded manifest with the secrets and cipher parameters
// derived from it.
type resolvedWorkspace struct {
	workspace *workspace.Workspace
	secrets   SecretsParams
	cipher    cipher.Params
}

// resolveWorkspace loads the manifest at configPath (empty walks up from the
// working directory) and derives the secrets and cipher parameters.
func resolveWorkspace(configPath string) (*resolvedWorkspace, error) {
	repo, err := wsfilestore.New(wsfilestore.Params{
		Path:     configPath,
		Filename: defaultManifestFilename,
	})
	if err != nil {
		return nil, err
	}

	wsService, err := workspace.NewService(workspace.ServiceParams{
		Repository: repo,
	})
	if err != nil {
		return nil, err
	}
	ws, err := wsService.Load()
	if err != nil {
		return nil, err
	}

	return &resolvedWorkspace{
		workspace: ws,
		secrets:   resolveSecretsParams(ws),
		cipher:    resolveCipherParams(ws),
	}, nil
}

// resolveSecretsParams builds the secrets input from the workspace-level
// manifest secrets block: the resolved store and private-key paths and the
// default indent.
func resolveSecretsParams(ws *workspace.Workspace) SecretsParams {
	secretsPath := ws.Secrets.SecretsPath
	if secretsPath == "" {
		secretsPath = defaultSecretsFilename
	}
	resolvedSecretsPath := filex.ResolvePath(ws.Root, secretsPath)

	// The default key file sits beside the store; an explicit one is relative to
	// the manifest.
	keysPath := ws.Secrets.KeysPath
	if keysPath == "" {
		keysPath = filepath.Join(filepath.Dir(resolvedSecretsPath), defaultKeysFilename)
	} else {
		keysPath = filex.ResolvePath(ws.Root, keysPath)
	}

	indent := ws.Indent
	if indent < 2 || indent > 9 {
		indent = defaultIndent
	}

	return SecretsParams{
		SecretsPath:   resolvedSecretsPath,
		KeysPath:      keysPath,
		DefaultIndent: indent,
	}
}

// resolveCipherParams resolves the configured algorithm, defaulting when the
// manifest names none.
func resolveCipherParams(ws *workspace.Workspace) cipher.Params {
	algorithm := cipher.Algorithm(ws.Secrets.Cipher)
	if algorithm == "" {
		algorithm = defaultCipherAlgorithm
	}
	return cipher.Params{Algorithm: algorithm}
}
