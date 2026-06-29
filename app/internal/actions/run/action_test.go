package run

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// -------------------------------------------------------------------------------------

// TestExecuteInjectsEnv verifies the resolved environment reaches the child
// process under the default (no-overload) settings.
func TestExecuteInjectsEnv(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	in := &config.Input{ConfigPath: &path}
	err := execute(context.Background(), actionParams{
		Project:  "api-core",
		ExecArgs: []string{"printenv", "APP_NAME"},
	}, in, streams{Stdout: &stdout, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); got != "api-core\n" {
		t.Errorf("child APP_NAME = %q, want api-core", got)
	}
}

// -------------------------------------------------------------------------------------

// TestExecuteOverloadFromEnv verifies ENVX_OVERLOAD lets file values win over an
// OS env var even without the --overload flag.
func TestExecuteOverloadFromEnv(t *testing.T) {
	t.Setenv("APP_NAME", "from-os")
	t.Setenv("ENVX_OVERLOAD", "true")

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	in := &config.Input{ConfigPath: &path}
	err := execute(context.Background(), actionParams{
		Project:  "api-core",
		ExecArgs: []string{"printenv", "APP_NAME"},
	}, in, streams{Stdout: &stdout, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); got != "api-core\n" {
		t.Errorf("APP_NAME = %q, want api-core (file wins via ENVX_OVERLOAD)", got)
	}
}
