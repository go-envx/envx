//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/testutil"
)

const (
	// buildTimeout bounds compiling the binary under test.
	buildTimeout = 2 * time.Minute
	// commandTimeout bounds every envx invocation, including its children.
	commandTimeout = 30 * time.Second
	// waitDelay bounds draining output after an invocation is killed.
	waitDelay = 5 * time.Second
	// helperArg makes the test binary act as a portable child command.
	helperArg = "-envx-e2e-helper"
)

// envxPath is the binary built once by TestMain for the whole suite.
var envxPath string

// result is the observable outcome of one envx invocation.
type result struct {
	// stdout is everything the process wrote to standard output.
	stdout string
	// stderr is everything the process wrote to standard error.
	stderr string
	// code is the process exit status.
	code int
}

// TestMain dispatches helper invocations, otherwise builds the envx binary
// once, runs the suite, and removes the build.
func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == helperArg {
		os.Exit(runHelper(os.Args[2], os.Args[3:]))
	}
	os.Exit(runSuite(m))
}

// runHelper implements the child commands envx runs in tests: printenv KEY,
// exit CODE, and await-term (prints "ready", exits 42 on SIGTERM).
func runHelper(name string, args []string) int {
	switch name {
	case "printenv":
		value, ok := os.LookupEnv(args[0])
		if !ok {
			return 1
		}
		fmt.Println(value)
	case "exit":
		code, _ := strconv.Atoi(args[0])
		return code
	case "await-term":
		term := make(chan os.Signal, 1)
		signal.Notify(term, syscall.SIGTERM)
		fmt.Println("ready")
		<-term
		return 42
	default:
		return 2
	}
	return 0
}

// helper returns the run arguments (after "--") that start a helper command.
func helper(t *testing.T, args ...string) []string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{self, helperArg}, args...)
}

// runSuite builds envx into a temporary directory and runs the tests against it.
func runSuite(m *testing.M) int {
	dir, err := os.MkdirTemp("", "envx-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: create build dir: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()

	envxPath = filepath.Join(dir, "envx")
	if runtime.GOOS == "windows" {
		envxPath += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	build := exec.CommandContext(
		ctx, "go", "build",
		"-ldflags", "-X main.version=test",
		"-o", envxPath,
		"github.com/go-envx/envx/app/cmd/envx",
	)
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build envx: %v\n", err)
		return 1
	}
	return m.Run()
}

// command prepares envx in its own process group with a controlled environment
// (baseEnv plus env), a scratch working directory, and a deadline. Cleanup
// kills the whole process tree so children never outlive the test. The
// returned context reports whether the deadline cut the invocation short.
func command(
	t *testing.T,
	env []string,
	args ...string,
) (*exec.Cmd, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), commandTimeout)
	//nolint:gosec // the suite's own freshly built binary.
	cmd := exec.CommandContext(ctx, envxPath, args...)
	cmd.Dir = t.TempDir()
	cmd.Env = append(baseEnv(t.TempDir()), env...)
	isolate(cmd)
	cmd.Cancel = func() error { return killTree(cmd.Process.Pid) }
	cmd.WaitDelay = waitDelay
	t.Cleanup(func() {
		cancel()
		cleanupTree(cmd)
	})
	return cmd, ctx
}

// envx runs the binary with args and the extra env entries and returns its
// outcome. Only failure to start or a missed deadline fails the test.
func envx(t *testing.T, env []string, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd, ctx := command(t, env, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		t.Fatalf("envx %v: %v\nstderr:\n%s", args, ctxErr, stderr.String())
	}
	if _, ok := errors.AsType[*exec.ExitError](err); err != nil && !ok {
		t.Fatalf("envx %v: %v", args, err)
	}
	return result{
		stdout: stdout.String(),
		stderr: stderr.String(),
		code:   cmd.ProcessState.ExitCode(),
	}
}

// mustEnvx runs envx and fails the test unless it exits successfully.
func mustEnvx(t *testing.T, env []string, args ...string) result {
	t.Helper()
	res := envx(t, env, args...)
	if res.code != 0 {
		t.Fatalf("envx %v exited %d\nstderr:\n%s", args, res.code, res.stderr)
	}
	return res
}

// basicWorkspace copies the basic fixture and returns its manifest path.
func basicWorkspace(t *testing.T) string {
	t.Helper()
	return filepath.Join(testutil.CopyDir(t, "testdata/basic"), "envx.yaml")
}

// secretWorkspace creates a workspace whose app project references one
// encrypted secret (app/api_key = s3cr3t). It returns the manifest path and the
// ENVX_PRIVATE_KEY entry that decrypts it.
func secretWorkspace(t *testing.T) (cfg, privateKeyEnv string) {
	t.Helper()
	work := t.TempDir()
	cfg = filepath.Join(work, "envx.yaml")
	writeFile(t, cfg, "environments: [production]\n"+
		"secrets:\n  cipher: age\n"+
		"projects:\n  app:\n    includes: [env/app]\n")
	writeFile(
		t,
		filepath.Join(work, "env", "app.yaml"),
		"GREETING: hello\nAPI_KEY: secret://app/api_key\n",
	)

	mustEnvx(t, nil, "keypair", "generate", "app", "--config", cfg)
	mustEnvx(t, nil, "secrets", "set", "app", "api_key", "s3cr3t", "--config", cfg)
	privateKey, err := filex.Read(filepath.Join(work, "envx.keys"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg, "ENVX_PRIVATE_KEY=" + string(privateKey)
}

// writeFile writes body to path, creating parent directories.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
