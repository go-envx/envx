package run

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/fixtures"
)

// -------------------------------------------------------------------------------------
// TestExecuteOverloadFromEnv verifies ENVX_OVERLOAD lets file values win over an
// OS env var even without the --overload flag.
func TestExecuteOverloadFromEnv(t *testing.T) {
	t.Setenv("APP_NAME", "from-os")
	t.Setenv("ENVX_OVERLOAD", "true")

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	c := &actionConfig{ConfigPath: &path, Changed: noChange{}}
	err := execute(context.Background(), actionParams{
		Project:  "api-core",
		ExecArgs: []string{"printenv", "APP_NAME"},
		Stdout:   &stdout,
		Stderr:   io.Discard,
	}, c)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); got != "api-core\n" {
		t.Errorf("APP_NAME = %q, want api-core (file wins via ENVX_OVERLOAD)", got)
	}
}
