package envmerge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeYAML writes a YAML file into dir.
func writeYAML(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
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

// recordingFactory is a ValueResolverFactory that records how many resolvers it
// opened and the reveal policy of the last call, returning a caller-supplied
// resolver. It proves each operation opens exactly one fresh resolver and that
// construction opens none.
type recordingFactory struct {
	// calls counts how many times Resolver was invoked.
	calls int
	// reveal records the reveal policy of the most recent call.
	reveal bool
	// resolver is returned to the operation on each call.
	resolver ValueResolver
}

// Resolver records the call and returns the configured resolver.
func (f *recordingFactory) Resolver(reveal bool) (ValueResolver, error) {
	f.calls++
	f.reveal = reveal
	return f.resolver, nil
}

// mutableFactory returns a fresh resolver reflecting its current value on each
// call, so a test can prove no resolver state survives across operations.
type mutableFactory struct {
	// calls counts how many times Resolver was invoked.
	calls int
	// value is the plaintext the returned resolver maps "secret://x" to.
	value string
}

// Resolver returns a fresh resolver bound to the factory's current value.
func (f *mutableFactory) Resolver(bool) (ValueResolver, error) {
	f.calls++
	return fakeResolver{values: map[string]string{"secret://x": f.value}}, nil
}

// managerFor builds a Manager over a single namespace declaring development and
// production, without validating the environment at construction.
func managerFor(t *testing.T, params Params) *Manager {
	t.Helper()
	if params.Environments == nil {
		params.Environments = []string{"development", "production"}
	}
	manager, err := New(params)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return manager
}

// mergeEnv constructs a Manager from p and materializes its default environment,
// exercising the shared merge kernel exactly as a Manager operation does.
func mergeEnv(t *testing.T, p Params) (*Environment, error) {
	t.Helper()
	manager, err := New(p)
	if err != nil {
		return nil, err
	}
	return manager.Materialize("")
}
