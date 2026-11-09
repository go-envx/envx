package env

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
	"github.com/go-envx/envx/app/internal/lib/severity"
	"github.com/go-envx/envx/app/internal/lib/str"
)

// secretPrefix marks values the fake resolver treats as secret references.
const secretPrefix = "secret://"

// fakeNamespaces is an in-memory NamespaceReader keyed by include path.
type fakeNamespaces struct {
	// bases maps an include path to its base data.
	bases map[string]map[string]any
	// overlays maps an include path and environment to overlay data.
	overlays map[string]map[string]map[string]any
	// baseReads counts LoadBase calls.
	baseReads int
}

// fakeOverlays records SetOverlay calls.
type fakeOverlays struct {
	// include is the last include path written.
	include string
	// environment is the last environment written.
	environment string
	// key is the last key written.
	key string
	// text is the last text written.
	text string
}

// fakeResolver resolves "secret://" references from a plaintext table.
type fakeResolver struct {
	// reveal decrypts references instead of masking them.
	reveal bool
	// plaintext maps references to their plaintext.
	plaintext map[string]string
}

// fakeOpener opens fakeResolvers and records each call.
type fakeOpener struct {
	// plaintext is handed to every opened resolver.
	plaintext map[string]string
	// calls counts opened resolvers.
	calls int
	// reveal records the last reveal policy.
	reveal bool
	// host records the last host snapshot.
	host hostenv.Snapshot
}

// fakeExecutor records the command it would run.
type fakeExecutor struct {
	// command is the last command executed.
	command process.Command
	// calls counts executions.
	calls int
	// err is returned from every execution.
	err error
}

// newFakeNamespaces returns an empty namespace reader.
func newFakeNamespaces() *fakeNamespaces {
	return &fakeNamespaces{
		bases:    make(map[string]map[string]any),
		overlays: make(map[string]map[string]map[string]any),
	}
}

// base sets an include's base document from YAML.
func (f *fakeNamespaces) base(t *testing.T, include, text string) *fakeNamespaces {
	t.Helper()
	f.bases[include] = parseYAML(t, text)
	return f
}

// overlay sets an include's overlay document from YAML.
func (f *fakeNamespaces) overlay(
	t *testing.T, include, environment, text string,
) *fakeNamespaces {
	t.Helper()
	if f.overlays[include] == nil {
		f.overlays[include] = make(map[string]map[string]any)
	}
	f.overlays[include][environment] = parseYAML(t, text)
	return f
}

// LoadBase implements NamespaceReader.
func (f *fakeNamespaces) LoadBase(include string) (NamespaceData, error) {
	f.baseReads++
	data, ok := f.bases[include]
	if !ok {
		return NamespaceData{}, fmt.Errorf("loading %s.yaml: %w", include, os.ErrNotExist)
	}
	return NamespaceData{Data: data, SourcePath: include + ".yaml"}, nil
}

// LoadOverlay implements NamespaceReader.
func (f *fakeNamespaces) LoadOverlay(
	include, environment string,
) (NamespaceData, bool, error) {
	path := include + "." + environment + ".yaml"
	data, ok := f.overlays[include][environment]
	if !ok {
		return NamespaceData{SourcePath: path}, false, nil
	}
	return NamespaceData{Data: data, SourcePath: path}, true, nil
}

// SetOverlay implements OverlayWriter.
func (f *fakeOverlays) SetOverlay(
	include, environment, key, text string,
) (string, error) {
	f.include, f.environment, f.key, f.text = include, environment, key, text
	return include + "." + environment + ".yaml", nil
}

// Resolve implements ValueResolver.
func (r fakeResolver) Resolve(raw, _ string) (string, error) {
	if !strings.HasPrefix(raw, secretPrefix) || !r.reveal {
		return raw, nil
	}
	plaintext, ok := r.plaintext[raw]
	if !ok {
		return "", fmt.Errorf("secret %s not found", raw)
	}
	return plaintext, nil
}

// Evaluate implements ValueResolver.
func (r fakeResolver) Evaluate(raw, _ string) value.Evaluation {
	if !strings.HasPrefix(raw, secretPrefix) {
		return value.Evaluation{Kind: value.Config, Value: raw, Resolved: true}
	}
	eval := value.Evaluation{Kind: value.Secret, Value: raw}
	plaintext, ok := r.plaintext[raw]
	if !ok {
		eval.Diagnostic = diagnostic.Finding{
			Code:    diagnostic.SecretReferenceNotFound,
			Level:   severity.Error,
			Message: "not found",
		}
		return eval
	}
	if r.reveal {
		eval.Value, eval.Resolved = plaintext, true
	}
	return eval
}

// open implements Dependencies.OpenResolver.
func (o *fakeOpener) open(reveal bool, host hostenv.Snapshot) (ValueResolver, error) {
	o.calls++
	o.reveal, o.host = reveal, host
	return fakeResolver{reveal: reveal, plaintext: o.plaintext}, nil
}

// run implements Executor.
func (e *fakeExecutor) run(command process.Command) error {
	e.calls++
	e.command = command
	return e.err
}

// parseYAML decodes a dedented YAML mapping.
func parseYAML(t *testing.T, text string) map[string]any {
	t.Helper()
	m := make(map[string]any)
	if err := yaml.Unmarshal([]byte(str.Dedent(text)), &m); err != nil {
		t.Fatalf("parsing YAML: %v", err)
	}
	return m
}

// hostOf returns a snapshot source over KEY=value pairs.
func hostOf(environ ...string) func() hostenv.Snapshot {
	return func() hostenv.Snapshot { return hostenv.From(environ) }
}
