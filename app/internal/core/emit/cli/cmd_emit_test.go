package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/emit"
	"github.com/go-envx/envx/app/internal/core/emit/format"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/value"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/testutil"
)

// testConfig is the manifest path every command test passes with --config.
const testConfig = "/ws/envx.yaml"

// fakeFactory builds an emit service whose explainer is the factory itself,
// returning entries for project "api".
type fakeFactory struct {
	// configPath records the manifest path the command requested.
	configPath string
	// err, when set, is returned instead of a service.
	err error
	// entries are returned by every explain request.
	entries []env.ExplainEntry
	// explains records every explain request.
	explains []env.ExplainParams
}

// NewEmitService implements Factory.
func (f *fakeFactory) NewEmitService(configPath string) (*emit.Service, error) {
	f.configPath = configPath
	if f.err != nil {
		return nil, f.err
	}
	return emit.New(emit.Dependencies{
		Env:      f,
		Renderer: format.Renderer{},
	})
}

// Explain implements emit.Explainer.
func (f *fakeFactory) Explain(params env.ExplainParams) (env.ExplainResult, error) {
	f.explains = append(f.explains, params)
	return env.ExplainResult{Project: "api", Entries: f.entries}, nil
}

// TestMain keeps ambient ENVX_* settings out of the command's flags.
func TestMain(m *testing.M) {
	testutil.UnsetEnvPrefix("ENVX_")
	os.Exit(m.Run())
}

// resolvedEntries is one plain and one secret-derived resolved value.
func resolvedEntries() []env.ExplainEntry {
	return []env.ExplainEntry{
		{
			Key:        "APP_NAME",
			Evaluation: value.Evaluation{Value: "myapp", Resolved: true},
		},
		{
			Key:        "DB_PASSWORD",
			Evaluation: value.Evaluation{Value: "top-secret", Resolved: true},
			Secret:     true,
		},
	}
}

// executeEmit builds the emit command, executes it with args, and returns its
// captured stdout and stderr.
func executeEmit(
	t *testing.T, factory Factory, args ...string,
) (stdout, stderr string, err error) {
	t.Helper()

	cmd := NewEmitCommand(factory)
	workspacecli.BindConfig(cmd)
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", testConfig}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// TestEmitCommandWritesStdout verifies --config, the project, and settings
// reach the service and the render goes to stdout with nothing on stderr.
func TestEmitCommandWritesStdout(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{entries: resolvedEntries()}
	stdout, stderr, err := executeEmit(
		t, factory, "api", "--target", "dotenv", "--env", "production",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if factory.configPath != testConfig {
		t.Errorf("factory configPath = %q, want %q", factory.configPath, testConfig)
	}
	if len(factory.explains) != 1 {
		t.Fatalf("explains = %d, want 1", len(factory.explains))
	}
	params := factory.explains[0]
	if params.Project != "api" {
		t.Errorf("Project = %q, want api", params.Project)
	}
	if params.Options.Env == nil || *params.Options.Env != "production" {
		t.Errorf("Options.Env = %v, want production", params.Options.Env)
	}
	if !strings.Contains(stdout, "APP_NAME=myapp\n") {
		t.Errorf("stdout missing APP_NAME:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing when emitting to stdout", stderr)
	}
}

// TestEmitCommandRenderFlags verifies --target, --only, --name, and --key reach
// the renderer.
func TestEmitCommandRenderFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// args are the flags after the project.
		args []string
		// want lists substrings of the expected output.
		want []string
		// reject lists substrings the output must not contain.
		reject []string
	}{
		{
			name:   "only config",
			args:   []string{"--target", "json", "--only", "config"},
			want:   []string{`"APP_NAME": "myapp"`},
			reject: []string{"DB_PASSWORD"},
		},
		{
			name: "name",
			args: []string{"--target", "k8s", "--only", "config", "--name", "web"},
			want: []string{"kind: ConfigMap", "name: web\n"},
		},
		{
			name: "key",
			args: []string{
				"--target", "k8s-bundle", "--only", "config", "--key", "app.env",
			},
			want: []string{"app.env: |"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := &fakeFactory{entries: resolvedEntries()}
			args := append([]string{"api"}, tt.args...)
			stdout, _, err := executeEmit(t, factory, args...)
			if err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("output missing %q:\n%s", want, stdout)
				}
			}
			for _, reject := range tt.reject {
				if strings.Contains(stdout, reject) {
					t.Errorf("output unexpectedly contains %q:\n%s", reject, stdout)
				}
			}
		})
	}
}

// TestEmitCommandRequiresTarget verifies emit rejects an invocation with no
// --target before calling the factory.
func TestEmitCommandRequiresTarget(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{}
	_, _, err := executeEmit(t, factory, "api")
	if err == nil || !strings.Contains(err.Error(), `required flag(s) "target"`) {
		t.Fatalf("err = %v, want a required --target error", err)
	}
	if factory.configPath != "" {
		t.Error("factory was called despite the missing --target")
	}
}

// TestEmitCommandUnknownTarget verifies an unknown --target fails before any
// value is explained or output written.
func TestEmitCommandUnknownTarget(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{entries: resolvedEntries()}
	stdout, _, err := executeEmit(t, factory, "api", "--target", "toml")
	if !errors.Is(err, format.ErrUnknownTarget) {
		t.Fatalf("err = %v, want ErrUnknownTarget", err)
	}
	if len(factory.explains) != 0 || stdout != "" {
		t.Errorf(
			"explains = %d, stdout = %q; want neither", len(factory.explains), stdout,
		)
	}
}

// TestEmitCommandFailsClosedOnUnresolved verifies an unresolved value aborts
// emit with no output.
func TestEmitCommandFailsClosedOnUnresolved(t *testing.T) {
	t.Parallel()

	entries := append(resolvedEntries(), env.ExplainEntry{Key: "BROKEN"})
	stdout, _, err := executeEmit(
		t, &fakeFactory{entries: entries}, "api", "--target", "dotenv",
	)
	if !errors.Is(err, env.ErrUnresolved) {
		t.Fatalf("err = %v, want ErrUnresolved", err)
	}
	if stdout != "" {
		t.Errorf("emit wrote output despite the failure: %q", stdout)
	}
}

// TestEmitCommandFactoryError verifies a factory failure aborts emit before any
// output is written.
func TestEmitCommandFactoryError(t *testing.T) {
	t.Parallel()

	want := errors.New("factory failure")
	stdout, _, err := executeEmit(
		t, &fakeFactory{err: want}, "api", "--target", "dotenv",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if stdout != "" {
		t.Errorf("emit wrote output despite the factory failure: %q", stdout)
	}
}

// TestEmitCommandWritesFile verifies --output writes a private file, confirms
// on stdout, and warns on stderr only when the file carries secrets.
func TestEmitCommandWritesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.env")
	stdout, stderr, err := executeEmit(
		t, &fakeFactory{entries: resolvedEntries()},
		"api", "--target", "dotenv", "--output", outPath,
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if strings.Contains(stdout, "APP_NAME=") {
		t.Errorf("render went to stdout despite --output: %q", stdout)
	}
	if !strings.Contains(stdout, "Wrote api config and secrets to "+outPath) {
		t.Errorf("stdout = %q, want the write confirmation", stdout)
	}
	if !strings.Contains(stderr, "WARNING:") ||
		!strings.Contains(stderr, "do not commit") {
		t.Errorf("stderr = %q, want a do-not-commit warning", stderr)
	}
	data, err := filex.Read(outPath)
	if err != nil {
		t.Fatalf("Read(%q): %v", outPath, err)
	}
	if !strings.Contains(string(data), "APP_NAME=myapp\n") {
		t.Errorf("output file missing APP_NAME:\n%s", data)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("output file mode = %o, want 600", perm)
	}

	configPath := filepath.Join(dir, "config.json")
	stdout, stderr, err = executeEmit(
		t, &fakeFactory{entries: resolvedEntries()},
		"api", "--target", "json", "--only", "config", "--output", configPath,
	)
	if err != nil {
		t.Fatalf("Execute() config only: %v", err)
	}
	if !strings.Contains(stdout, "Wrote api config to") {
		t.Errorf("stdout = %q, want the config write confirmation", stdout)
	}
	if stderr != "" {
		t.Errorf("config-only file should not warn, got %q", stderr)
	}

	bundlePath := filepath.Join(dir, "secrets.yaml")
	stdout, _, err = executeEmit(
		t, &fakeFactory{entries: resolvedEntries()},
		"api", "--target", "k8s-bundle", "--only", "secrets", "--output", bundlePath,
	)
	if err != nil {
		t.Fatalf("Execute() secrets bundle: %v", err)
	}
	if !strings.Contains(stdout, "Wrote api secrets to "+bundlePath+". Read the docs") {
		t.Errorf("stdout = %q, want the bundle confirmation", stdout)
	}
}

// TestEmitCommandMissingOutputDir verifies emit refuses a missing output
// directory before calling the factory.
func TestEmitCommandMissingOutputDir(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{entries: resolvedEntries()}
	missing := filepath.Join(t.TempDir(), "nope", "out.env")
	_, _, err := executeEmit(
		t, factory, "api", "--target", "dotenv", "--output", missing,
	)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v, want a missing-directory error", err)
	}
	if factory.configPath != "" {
		t.Error("factory was called despite the missing output directory")
	}
}
