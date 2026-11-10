package cli

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/emit"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/pack"
	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/validate"
	"github.com/go-envx/envx/app/internal/lib/stdio"
	"github.com/go-envx/envx/app/internal/testutil"
)

// errNoService is returned by every fakeFactory constructor.
var errNoService = errors.New("no service")

// fakeFactory satisfies appFactory without assembling any service, recording
// the manifest path each constructor receives.
type fakeFactory struct {
	// configPaths records the manifest path of every constructor call.
	configPaths []string
	// scaffolds counts NewScaffoldService calls.
	scaffolds int
}

// TestMain keeps ambient ENVX_* settings out of the commands' flags.
func TestMain(m *testing.M) {
	testutil.UnsetEnvPrefix("ENVX_")
	os.Exit(m.Run())
}

// NewEmitService implements appFactory.
func (f *fakeFactory) NewEmitService(configPath string) (*emit.Service, error) {
	f.configPaths = append(f.configPaths, configPath)
	return nil, errNoService
}

// NewEnvService implements appFactory.
func (f *fakeFactory) NewEnvService(configPath string) (*env.Service, error) {
	f.configPaths = append(f.configPaths, configPath)
	return nil, errNoService
}

// NewPackService implements appFactory.
func (f *fakeFactory) NewPackService(configPath string) (*pack.Service, error) {
	f.configPaths = append(f.configPaths, configPath)
	return nil, errNoService
}

// NewScaffoldService implements appFactory.
func (f *fakeFactory) NewScaffoldService() (*scaffold.Service, error) {
	f.scaffolds++
	return nil, errNoService
}

// NewSecretsService implements appFactory.
func (f *fakeFactory) NewSecretsService(configPath string) (*secrets.Service, error) {
	f.configPaths = append(f.configPaths, configPath)
	return nil, errNoService
}

// NewValidateService implements appFactory.
func (f *fakeFactory) NewValidateService(configPath string) (*validate.Service, error) {
	f.configPaths = append(f.configPaths, configPath)
	return nil, errNoService
}

// execCmd builds a root command wired to fresh stdout/stderr buffers, sets the
// given args, and executes it. It returns the captured buffers and any error.
func execCmd(
	factory appFactory, args ...string,
) (stdout, stderr *bytes.Buffer, err error) {
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	cmd := NewRootCommand(BuildInfo{Version: "test"}, factory, stdio.Streams{
		Out:    stdout,
		ErrOut: stderr,
	})
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// TestRootShowsHelp verifies bare invocation prints help and does not require a
// manifest.
func TestRootShowsHelp(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{}
	stdout, _, err := execCmd(factory)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "envx") {
		t.Errorf("expected help output, got %q", stdout.String())
	}
	if len(factory.configPaths) != 0 || factory.scaffolds != 0 {
		t.Error("help built a service")
	}
}

// TestVersionFlag verifies --version prints the injected version.
func TestVersionFlag(t *testing.T) {
	t.Parallel()

	stdout, _, err := execCmd(&fakeFactory{}, "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.HasPrefix(out, "envx version test\n") {
		t.Errorf("version output = %q, want it to start with %q", out, "envx version test\n")
	}
	if !strings.Contains(out, "commit:") {
		t.Errorf("version output %q missing commit metadata", out)
	}
}

// TestRootRoutesCommands verifies each command is registered and reaches the
// factory with the persistent --config path, returning its error unchanged
// without a usage dump.
func TestRootRoutesCommands(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"get", "app", "KEY"},
		{"set", "env/app", "KEY", "value"},
		{"explain", "app"},
		{"diff", "app", "development", "production"},
		{"run", "app", "--", "true"},
		{"emit", "app", "--target", "dotenv"},
		{"pack", "--out", "dist"},
		{"validate"},
		{"secrets", "get", "group", "key"},
		{"keypair", "inspect", "group"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()

			factory := &fakeFactory{}
			stdout, stderr, err := execCmd(
				factory, append([]string{"--config", "/ws/envx.yaml"}, args...)...,
			)
			if !errors.Is(err, errNoService) {
				t.Fatalf("err = %v, want errNoService", err)
			}
			if strings.Contains(stdout.String()+stderr.String(), "Usage:") {
				t.Error("a runtime error printed usage")
			}
			want := []string{"/ws/envx.yaml"}
			if !slices.Equal(factory.configPaths, want) {
				t.Errorf("configPaths = %q, want %q", factory.configPaths, want)
			}
		})
	}
}

// TestRootRoutesCreate verifies the scaffold command needs no manifest.
func TestRootRoutesCreate(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{}
	_, _, err := execCmd(factory, "create", "quick-start", "--target-dir", t.TempDir())
	if !errors.Is(err, errNoService) {
		t.Fatalf("err = %v, want errNoService", err)
	}
	if factory.scaffolds != 1 {
		t.Errorf("scaffolds = %d, want 1", factory.scaffolds)
	}
}

// TestRootInstallsStreams verifies subcommands inherit the given streams, and
// nil streams are normalized rather than falling back to the process streams.
func TestRootInstallsStreams(t *testing.T) {
	t.Parallel()

	in := strings.NewReader("")
	var out, errOut bytes.Buffer
	root := NewRootCommand(BuildInfo{}, &fakeFactory{}, stdio.Streams{
		In:     in,
		Out:    &out,
		ErrOut: &errOut,
	})
	run, _, err := root.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run): %v", err)
	}
	if run.InOrStdin() != in || run.OutOrStdout() != &out || run.ErrOrStderr() != &errOut {
		t.Error("subcommand streams are not the given streams")
	}

	empty := NewRootCommand(BuildInfo{}, &fakeFactory{}, stdio.Streams{})
	if empty.InOrStdin() == os.Stdin ||
		empty.OutOrStdout() == os.Stdout ||
		empty.ErrOrStderr() == os.Stderr {
		t.Error("nil streams fell back to the process streams")
	}
}
