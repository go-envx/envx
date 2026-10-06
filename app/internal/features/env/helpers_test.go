package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

type fakeNamespaceRepository struct {
	mu          sync.RWMutex
	bases       map[string]NamespaceData
	overlays    map[string]map[string]NamespaceData
	parseErrors map[string]error
	baseErr     error
	overlayErr  error
}

func newFakeNamespaceRepository() *fakeNamespaceRepository {
	return &fakeNamespaceRepository{
		bases:       make(map[string]NamespaceData),
		overlays:    make(map[string]map[string]NamespaceData),
		parseErrors: make(map[string]error),
	}
}

var testRepo = newFakeNamespaceRepository()

func (r *fakeNamespaceRepository) LoadBase(includePath string) (NamespaceData, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.baseErr != nil {
		return NamespaceData{}, r.baseErr
	}
	clean := strings.TrimSuffix(includePath, ".yaml")
	baseFile := clean + ".yaml"
	if err, ok := r.parseErrors[baseFile]; ok {
		return NamespaceData{}, err
	}
	if data, ok := r.bases[clean]; ok {
		return data, nil
	}
	return NamespaceData{}, fmt.Errorf(
		"loading base file %s: %w", baseFile, os.ErrNotExist,
	)
}

func (r *fakeNamespaceRepository) LoadOverlay(
	includePath, environment string,
) (NamespaceData, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.overlayErr != nil {
		return NamespaceData{}, false, r.overlayErr
	}
	clean := strings.TrimSuffix(includePath, ".yaml")
	envFile := clean + "." + environment + ".yaml"
	if err, ok := r.parseErrors[envFile]; ok {
		return NamespaceData{}, false, err
	}
	if envMap, ok := r.overlays[clean]; ok {
		if data, ok := envMap[environment]; ok {
			return data, true, nil
		}
	}
	return NamespaceData{SourcePath: envFile}, false, nil
}

func (r *fakeNamespaceRepository) SetOverlay(
	includePath, environment, key, value string,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	clean := strings.TrimSuffix(includePath, ".yaml")
	envFile := clean + "." + environment + ".yaml"
	if r.overlays[clean] == nil {
		r.overlays[clean] = make(map[string]NamespaceData)
	}
	data := r.overlays[clean][environment].Data
	if data == nil {
		data = make(map[string]any)
	}
	data[key] = value
	r.overlays[clean][environment] = NamespaceData{
		Data:       data,
		SourcePath: envFile,
	}
	return envFile, nil
}

// writeYAML writes a YAML file into dir and updates the test fake repository.
func writeYAML(t *testing.T, dir, name, body string) {
	t.Helper()
	filePath := filepath.Join(dir, name)
	if err := os.WriteFile(filePath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	testRepo.mu.Lock()
	defer testRepo.mu.Unlock()

	var m map[string]any
	if err := yaml.Unmarshal([]byte(body), &m); err != nil {
		testRepo.parseErrors[filePath] = fmt.Errorf("loading %s: %w", filePath, err)
		return
	}
	if m == nil {
		m = make(map[string]any)
	}

	stem := strings.TrimSuffix(name, ".yaml")
	if strings.Contains(stem, ".") {
		parts := strings.SplitN(stem, ".", 2)
		inc := filepath.Join(dir, parts[0])
		envName := parts[1]
		if testRepo.overlays[inc] == nil {
			testRepo.overlays[inc] = make(map[string]NamespaceData)
		}
		testRepo.overlays[inc][envName] = NamespaceData{
			Data:       m,
			SourcePath: filePath,
		}
	} else {
		inc := filepath.Join(dir, stem)
		testRepo.bases[inc] = NamespaceData{
			Data:       m,
			SourcePath: filePath,
		}
	}
}

// fakeResolver implements Resolver for testing the reference-resolution step: it
// maps known reference values to results, fails designated values, and passes
// everything else through unchanged.
type fakeResolver struct {
	values map[string]string
	fail   string
	// failAll fails every value not present in values, so a test can dangle
	// several references at once.
	failAll bool
}

// Resolve maps value to its result, erroring on the designated failure value and
// returning unknown values unchanged.
func (f fakeResolver) Resolve(value, _ string) (string, error) {
	if v, ok := f.values[value]; ok {
		return v, nil
	}
	if f.failAll || (f.fail != "" && value == f.fail) {
		return "", errors.New("resolve failed")
	}
	return value, nil
}

// recordingFactory is a SecretsService that records how many resolvers it
// opened and the reveal policy of the last call, returning a caller-supplied
// resolver. It proves each operation opens exactly one fresh resolver and that
// construction opens none.
type recordingFactory struct {
	// calls counts how many times OpenResolver was invoked.
	calls int
	// reveal records the reveal policy of the most recent call.
	reveal bool
	// resolver is returned to the operation on each call.
	resolver ValueResolver
}

// OpenResolver records the call and returns the configured resolver.
func (f *recordingFactory) OpenResolver(reveal bool) (ValueResolver, error) {
	f.calls++
	f.reveal = reveal
	return f.resolver, nil
}

// mutableFactory returns a fresh resolver reflecting its current value on each
// call, so a test can prove no resolver state survives across operations.
type mutableFactory struct {
	// calls counts how many times OpenResolver was invoked.
	calls int
	// value is the plaintext the returned resolver maps "secret://x" to.
	value string
}

// OpenResolver returns a fresh resolver bound to the factory's current value.
func (f *mutableFactory) OpenResolver(bool) (ValueResolver, error) {
	f.calls++
	return fakeResolver{values: map[string]string{"secret://x": f.value}}, nil
}

type fakeHostEnv map[string]string

func (f fakeHostEnv) Get(name string) (string, bool) {
	val, ok := f[name]
	return val, ok
}

func (f fakeHostEnv) All() map[string]string {
	cp := make(map[string]string, len(f))
	for k, v := range f {
		cp[k] = v
	}
	return cp
}

// managerFor builds a Manager over a single namespace declaring development and
// production, without validating the environment at construction.
func managerFor(t *testing.T, params ServiceParams) *Service {
	t.Helper()
	if params.Config.Environments == nil {
		params.Config.Environments = []string{"development", "production"}
	}
	if params.Repository == nil {
		params.Repository = testRepo
	}
	manager, err := NewService(params)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return manager
}

// mergeEnv constructs a Manager from p and materializes its default environment,
// exercising the shared merge kernel exactly as a Manager operation does.
//
//nolint:gocritic // Test helper matches Params signature.
func mergeEnv(t *testing.T, p ServiceParams) (*Environment, error) {
	t.Helper()
	if p.Repository == nil {
		p.Repository = testRepo
	}
	manager, err := NewService(p)
	if err != nil {
		return nil, err
	}
	result, err := manager.Materialize(MaterializeParams{})
	if err != nil {
		return nil, err
	}
	return result.Environment, nil
}

// materializeEnv materializes environment through manager, failing the test on
// error and returning the resulting environment for assertions.
func materializeEnv(t *testing.T, manager *Service, environment string) *Environment {
	t.Helper()
	result, err := manager.Materialize(MaterializeParams{Environment: environment})
	if err != nil {
		t.Fatalf("Materialize(%q): %v", environment, err)
	}
	return result.Environment
}
