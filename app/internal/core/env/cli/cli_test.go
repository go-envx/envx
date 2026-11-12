package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/workspace"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
	"github.com/go-envx/envx/app/internal/testutil"
	"github.com/spf13/cobra"
)

// testConfig is the manifest path every command test passes with --config.
const testConfig = "/ws/envx.yaml"

// appInclude is the absolute include path of the test project "app".
const appInclude = "/ws/env/postgres"

// fakeFactory builds a real environment service over in-memory namespaces,
// recording overlay writes and child processes instead of touching the host.
type fakeFactory struct {
	// configPath records the manifest path the command requested.
	configPath string
	// err, when set, is returned instead of a service.
	err error
	// namespaces holds the namespace files the service reads.
	namespaces fakeNamespaces
	// host is the host environment as KEY=value entries.
	host []string
	// reveals records the reveal policy of every opened resolver.
	reveals []bool
	// overlays records every overlay write.
	overlays []overlayWrite
	// runs records every child process.
	runs []process.Command
	// runErr, when set, is returned by every child process.
	runErr error
}

// fakeNamespaces maps an include path, or "<include>.<environment>" for an
// overlay, to its parsed data.
type fakeNamespaces map[string]map[string]any

// overlayWrite is one recorded SetOverlay call.
type overlayWrite struct {
	// include is the absolute include path.
	include string
	// environment is the overlay environment.
	environment string
	// key is the dotted key written.
	key string
	// text is the value written.
	text string
}

// commandCase is one environment command with arguments that succeed against
// newFakeFactory.
type commandCase struct {
	// name labels the command.
	name string
	// new builds the command.
	new func(Factory) *cobra.Command
	// args are the positional arguments and flags.
	args []string
}

// TestMain keeps ambient ENVX_* settings out of the commands' flags.
func TestMain(m *testing.M) {
	testutil.UnsetEnvPrefix("ENVX_")
	os.Exit(m.Run())
}

// newFakeFactory returns a factory whose project "app" declares host and port,
// with environment overlays that change host and each add one key.
func newFakeFactory() *fakeFactory {
	return &fakeFactory{
		namespaces: fakeNamespaces{
			appInclude:                  {"host": "localhost", "port": "5432"},
			appInclude + ".development": {"host": "dev-db.local", "debug": "true"},
			appInclude + ".production":  {"host": "prod-db.internal", "pool": "20"},
		},
	}
}

// NewEnvService implements Factory.
func (f *fakeFactory) NewEnvService(configPath string) (*env.Service, error) {
	f.configPath = configPath
	if f.err != nil {
		return nil, f.err
	}
	ws := workspace.Workspace{
		Root:         "/ws",
		Environments: []string{"development", "production"},
		Projects: map[string]workspace.Project{
			"app": {
				Includes:     []string{"env/postgres"},
				IncludePaths: []string{appInclude},
			},
		},
	}
	return env.New(ws, env.Dependencies{
		EnvNamespaces: f.namespaces,
		EnvOverlays:   f,
		HostEnv: func() hostenv.Snapshot {
			return hostenv.From(f.host)
		},
		NewValueResolver: func(
			params env.ResolverParams,
		) (env.ValueResolver, error) {
			f.reveals = append(f.reveals, params.Reveal)
			// A nil resolver is identity resolution.
			return nil, nil
		},
		ProcessRunner: func(command process.Command) error {
			f.runs = append(f.runs, command)
			return f.runErr
		},
	})
}

// SetOverlay implements env.OverlayWriter.
func (f *fakeFactory) SetOverlay(
	include, environment, key, text string,
) (string, error) {
	f.overlays = append(f.overlays, overlayWrite{
		include:     include,
		environment: environment,
		key:         key,
		text:        text,
	})
	return include + "." + environment + ".yaml", nil
}

// LoadBase implements env.NamespaceReader.
func (n fakeNamespaces) LoadBase(include string) (env.NamespaceData, error) {
	data, ok := n[include]
	if !ok {
		return env.NamespaceData{}, fmt.Errorf("no namespace %q", include)
	}
	return env.NamespaceData{Data: data, SourcePath: include + ".yaml"}, nil
}

// LoadOverlay implements env.NamespaceReader.
func (n fakeNamespaces) LoadOverlay(
	include, environment string,
) (env.NamespaceData, bool, error) {
	data, ok := n[include+"."+environment]
	if !ok {
		return env.NamespaceData{}, false, nil
	}
	return env.NamespaceData{
		Data:       data,
		SourcePath: include + "." + environment + ".yaml",
	}, true, nil
}

// execute binds --config on cmd, runs it with args, and returns its captured
// stdout and stderr.
func execute(
	t *testing.T, cmd *cobra.Command, args ...string,
) (stdout, stderr string, err error) {
	t.Helper()

	workspacecli.BindConfig(cmd)
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", testConfig}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// assertGolden compares got with the named file under testdata.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	want, err := filex.Read(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}

// commandCases lists every environment command.
func commandCases() []commandCase {
	return []commandCase{
		{name: "get", new: NewGetCommand, args: []string{"app", "host"}},
		{name: "set", new: NewSetCommand, args: []string{"env/postgres", "host", "x"}},
		{name: "explain", new: NewExplainCommand, args: []string{"app"}},
		{
			name: "diff", new: NewDiffCommand,
			args: []string{"app", "development", "production"},
		},
		{name: "run", new: NewRunCommand, args: []string{"app", "--", "true"}},
	}
}

// TestCommandsPassConfigPath verifies every command builds its service from the
// --config path.
func TestCommandsPassConfigPath(t *testing.T) {
	t.Parallel()

	for _, tt := range commandCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := newFakeFactory()
			if _, _, err := execute(t, tt.new(factory), tt.args...); err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			if factory.configPath != testConfig {
				t.Errorf("configPath = %q, want %q", factory.configPath, testConfig)
			}
		})
	}
}

// TestCommandsReturnFactoryError verifies every command returns a factory
// failure unchanged and writes nothing.
func TestCommandsReturnFactoryError(t *testing.T) {
	t.Parallel()

	for _, tt := range commandCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want := errors.New("factory failure")
			factory := newFakeFactory()
			factory.err = want
			stdout, _, err := execute(t, tt.new(factory), tt.args...)
			if !errors.Is(err, want) {
				t.Fatalf("err = %v, want %v", err, want)
			}
			if stdout != "" || len(factory.runs) != 0 {
				t.Errorf("stdout = %q, runs = %d; want neither",
					stdout, len(factory.runs))
			}
		})
	}
}

// TestCommandsMapReveal verifies --reveal reaches the resolver of every command
// that accepts it.
func TestCommandsMapReveal(t *testing.T) {
	t.Parallel()

	for _, tt := range commandCases() {
		if tt.name == "set" || tt.name == "run" || tt.name == "diff" {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			masked := newFakeFactory()
			if _, _, err := execute(t, tt.new(masked), tt.args...); err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			revealed := newFakeFactory()
			args := append([]string{"--reveal"}, tt.args...)
			if _, _, err := execute(t, tt.new(revealed), args...); err != nil {
				t.Fatalf("Execute(--reveal): %v", err)
			}
			for _, reveal := range masked.reveals {
				if reveal {
					t.Error("resolver revealed without --reveal")
				}
			}
			for _, reveal := range revealed.reveals {
				if !reveal {
					t.Error("resolver masked despite --reveal")
				}
			}
			if len(masked.reveals) == 0 || len(revealed.reveals) == 0 {
				t.Error("no resolver was opened")
			}
		})
	}
}
