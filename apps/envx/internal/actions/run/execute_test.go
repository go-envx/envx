package run

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
)

// -------------------------------------------------------------------------------------
// loadGlobal loads the shared "basic" fixture into a root context for the
// development environment.
func loadGlobal(t *testing.T) config.Global {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	cfgPath := filepath.Join(
		filepath.Dir(thisFile), "..", "..", "..", "testdata", "basic", "envx.yaml",
	)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return config.Global{Config: cfg, Environment: "development"}
}

// -------------------------------------------------------------------------------------
// TestExecuteOverloadFromEnv verifies ENVX_OVERLOAD lets file values win over an
// OS env var even without the --overload flag.
func TestExecuteOverloadFromEnv(t *testing.T) {
	t.Setenv("APP_NAME", "from-os")
	t.Setenv("ENVX_OVERLOAD", "true")

	g := loadGlobal(t)
	var stdout bytes.Buffer
	c := &actionConfig{Global: &g, Changed: noChange{}}
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
