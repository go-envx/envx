package pack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/utils/filex"
)

// defaultIndent is the block indentation applied to a rewritten manifest whose
// source document has no detectable indentation of its own.
const defaultIndent = 2

// The bundle standardizes on envx's default filenames regardless of what the
// source workspace called its manifest and store, so every packed workspace is
// consumed by an identical command — `envx run --config <dir>/envx.yaml`. run
// auto-discovers envx.yaml and secrets.yaml is the default store location, so the
// bundle carries no custom paths (pack drops an explicit secrets path entirely).
const (
	bundleManifestName = "envx.yaml"
	bundleSecretsName  = "secrets.yaml"
)

// SecretsReader defines what pack consumes from the source secrets store: raw
// encrypted records, read without decrypting.
type SecretsReader interface {
	// GetSecret retrieves one stored secret record.
	GetSecret(group, key string) (secrets.SecretRecord, bool, error)
}

// SecretsWriter defines what pack consumes from the bundled secrets store: an
// atomic bulk write of the filtered records.
type SecretsWriter interface {
	// SetSecrets stores every record in one write.
	SetSecrets(records []secrets.SecretRecord) error
}

// ServiceParams provides dependencies to the bundling domain service.
type ServiceParams struct {
	// Workspace is the file-level layout bundles are copied from.
	Workspace Workspace
	// SecretsReader reads the workspace secrets store the bundle is filtered from.
	SecretsReader SecretsReader
	// NewSecretsWriter opens the bundled secrets store at the given path.
	NewSecretsWriter func(path string) (SecretsWriter, error)
}

// Service selects environment-scoped workspace files and writes them as a
// deployable bundle.
type Service struct {
	// params is the privately-owned configuration copied at construction.
	params ServiceParams
}

// NewService constructs a bundling domain service.
func NewService(params ServiceParams) (*Service, error) {
	if params.SecretsReader == nil {
		return nil, errors.New("secrets reader is required")
	}
	if params.NewSecretsWriter == nil {
		return nil, errors.New("secrets writer factory is required")
	}
	return &Service{params: params}, nil
}

// Pack selects the environment-scoped file set from the workspace and writes a
// per-project bundle into params.OutDir: one manifest at the bundle root, each
// selected project's namespace files (base and selected-env overlays) under its
// own <project>/ directory, and a single filtered secrets store at the root. It
// rewrites the manifest so its includes point at the per-project paths and its
// secrets path is dropped, excludes the private-key file, and decrypts nothing.
// It returns the written file set or the first error, having written no output on
// a selection error. An existing non-empty output directory is refused unless
// params.Force is set, in which case it is cleared just before the bundle is
// written.
func (s *Service) Pack(params PackParams) (PackResult, error) {
	ws := s.params.Workspace

	if strings.TrimSpace(params.OutDir) == "" {
		return PackResult{}, ErrOutputRequired
	}

	outDir, err := filepath.Abs(params.OutDir)
	if err != nil {
		return PackResult{}, fmt.Errorf(
			"resolving output directory %q: %w", params.OutDir, err,
		)
	}

	// Refuse an existing non-empty destination up front so a bundle never merges
	// into stale files; --force opts into replacing it. The directory itself is
	// not touched until just before the first write, so this check writes nothing.
	if !params.Force {
		if err := ensureOutDirEmpty(outDir); err != nil {
			return PackResult{}, err
		}
	}

	// Resolve the environment and project selections against the declared sets so
	// an unknown name fails before anything is written.
	environments, err := selectEnvironments(ws.Environments, params.Environments)
	if err != nil {
		return PackResult{}, err
	}
	projects, err := selectProjects(ws.Projects, params.Projects)
	if err != nil {
		return PackResult{}, err
	}

	// The bundle always uses envx's default filenames, whatever the source
	// workspace called them. Reserve those names (and their stems) so no project
	// directory collides with a root bundle file.
	manifestName := bundleManifestName
	secretsName := ""
	if ws.SecretsPath != "" && exists(ws.SecretsPath) {
		secretsName = bundleSecretsName
	}
	reserved := []string{manifestName, stemOf(manifestName)}
	if secretsName != "" {
		reserved = append(reserved, secretsName, stemOf(secretsName))
	}
	dirs, err := assignProjectDirs(projects, reserved)
	if err != nil {
		return PackResult{}, err
	}

	// Discover each project's namespaces and the source files each contributes for
	// the selected environments, and assign filenames within the project directory.
	bundles, err := planProjects(ws.Root, projects, environments, dirs)
	if err != nil {
		return PackResult{}, err
	}

	// Build the per-project copy plan for the namespace files.
	items := planFiles(bundles)

	// Rewrite the manifest so each project's includes resolve against its bundle
	// directory and any explicit secrets path is dropped.
	manifestData, err := filex.Read(ws.ManifestPath)
	if err != nil {
		return PackResult{}, fmt.Errorf("reading manifest %s: %w", ws.ManifestPath, err)
	}
	rewritten, err := rewriteManifest(manifestData, bundles)
	if err != nil {
		return PackResult{}, err
	}

	// Clear a forced destination only now that every fallible pre-write step has
	// succeeded, so a --force run that fails earlier leaves any existing bundle
	// untouched. RemoveAll is a no-op when the directory is absent or empty.
	if params.Force {
		if err := os.RemoveAll(outDir); err != nil {
			return PackResult{}, fmt.Errorf(
				"clearing output directory %s: %w", outDir, err,
			)
		}
	}

	// Write the bundle: the rewritten manifest first, then every planned file.
	manifestTarget := filepath.Join(outDir, manifestName)
	if err := writeFile(manifestTarget, rewritten); err != nil {
		return PackResult{}, err
	}
	written := []string{manifestName}
	for _, item := range items {
		if err := copyInto(outDir, item); err != nil {
			return PackResult{}, err
		}
		written = append(written, item.dest)
	}

	// Copy only the secrets the selected projects' namespaces reference, dropping
	// every public key so the bundle is run-only. The store stays a single shared
	// file at the bundle root, filtered to the union of the selected projects'
	// references; a workspace with no referenced secret carries no store at all.
	if secretsName != "" {
		copied, err := s.copySecrets(filepath.Join(outDir, secretsName), bundles)
		if err != nil {
			return PackResult{}, err
		}
		if copied {
			written = append(written, secretsName)
		}
	}
	sort.Strings(written)

	projectNames := make([]string, len(projects))
	for i, project := range projects {
		projectNames[i] = project.Name
	}

	return PackResult{
		OutDir:       outDir,
		ManifestFile: manifestName,
		Files:        written,
		Environments: environments,
		Projects:     projectNames,
	}, nil
}

// copySecrets writes the secrets the bundled namespaces reference into a new
// store at target, reporting whether a store was written. It reads each record
// from the source store without decrypting, and skips a reference the store does
// not hold, so a dangling reference surfaces at run time rather than here.
func (s *Service) copySecrets(target string, bundles []projectBundle) (bool, error) {
	referenced, err := scanReferences(allIncludeFiles(bundles))
	if err != nil {
		return false, err
	}

	var records []secrets.SecretRecord
	for _, ref := range referenced {
		record, found, err := s.params.SecretsReader.GetSecret(ref.Group, ref.Key)
		if err != nil {
			return false, fmt.Errorf(
				"reading secret %s/%s from %s: %w",
				ref.Group, ref.Key, s.params.Workspace.SecretsPath, err,
			)
		}
		if found {
			records = append(records, record)
		}
	}
	if len(records) == 0 {
		return false, nil
	}

	writer, err := s.params.NewSecretsWriter(target)
	if err != nil {
		return false, fmt.Errorf("creating bundled secrets store %s: %w", target, err)
	}
	if err := writer.SetSecrets(records); err != nil {
		return false, fmt.Errorf("writing bundled secrets store %s: %w", target, err)
	}
	return true, nil
}
