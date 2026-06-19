package run

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/fixtures"
)

// -------------------------------------------------------------------------------------
// noChange is a config.FlagSet stub reporting that no flag was set.
type noChange struct{}

// -------------------------------------------------------------------------------------
// Changed always reports false.
func (noChange) Changed(string) bool { return false }

// -------------------------------------------------------------------------------------
// resolveBasic loads the shared "basic" fixture and resolves the api-core
// environment for the development environment.
func resolveBasic(t *testing.T) *engine.Result {
	t.Helper()
	cfg, err := config.Load(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	env, err := engine.ResolveEnv(&engine.Request{
		Config:   cfg,
		Project:  "api-core",
		Settings: engine.Settings{Env: "development"},
	})
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	return env
}

// -------------------------------------------------------------------------------------
// TestRunActionInjectsEnv verifies the resolved environment reaches the child
// process when overload lets file values win.
func TestRunActionInjectsEnv(t *testing.T) {
	t.Parallel()

	env := resolveBasic(t)
	var stdout bytes.Buffer
	err := runAction(
		context.Background(), env,
		actionParams{
			ExecArgs: []string{"printenv", "APP_NAME"},
			Stdout:   &stdout,
			Stderr:   io.Discard,
		},
		true,
	)
	if err != nil {
		t.Fatalf("runAction: %v", err)
	}
	if got := stdout.String(); got != "api-core\n" {
		t.Errorf("child APP_NAME = %q, want api-core", got)
	}
}
