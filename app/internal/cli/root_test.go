package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/emit"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/pack"
	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/validate"
	"github.com/go-envx/envx/app/internal/lib/stdio"
)

// errNoService is returned by every fakeFactory constructor.
var errNoService = errors.New("no service")

// fakeFactory satisfies appFactory without assembling any service.
type fakeFactory struct{}

func (fakeFactory) NewEmitService(string) (*emit.Service, error) {
	return nil, errNoService
}

func (fakeFactory) NewEnvService(string) (*env.Service, error) {
	return nil, errNoService
}

func (fakeFactory) NewPackService(string) (*pack.Service, error) {
	return nil, errNoService
}

func (fakeFactory) NewScaffoldService() (*scaffold.Service, error) {
	return nil, errNoService
}

func (fakeFactory) NewSecretsService(string) (*secrets.Service, error) {
	return nil, errNoService
}

func (fakeFactory) NewValidateService(string) (*validate.Service, error) {
	return nil, errNoService
}

// execCmd builds a root command wired to fresh stdout/stderr buffers, sets the
// given args, and executes it. It returns the captured buffers and any error.
func execCmd(args ...string) (stdout, stderr *bytes.Buffer, err error) {
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	cmd := NewRootCommand(BuildInfo{Version: "test"}, fakeFactory{}, stdio.Streams{
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

	stdout, _, err := execCmd()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "envx") {
		t.Errorf("expected help output, got %q", stdout.String())
	}
}

// TestVersionFlag verifies --version prints the injected version.
func TestVersionFlag(t *testing.T) {
	t.Parallel()

	stdout, _, err := execCmd("--version")
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
