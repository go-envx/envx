// Command envx is the entry point for the envx CLI. It executes the root
// command and maps errors to exit codes.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-envx/envx/app/internal/cli"
	"github.com/go-envx/envx/app/internal/exitcode"
)

// Build metadata is injected at link time via -ldflags (see .goreleaser.yaml);
// the defaults below apply to local `go run` builds.
var (
	// version is the semantic version or VCS tag of the build.
	version = "0.0.0-dev"
	// commit is the VCS revision the binary was built from.
	commit = "unknown"
	// date is the build timestamp.
	date = "unknown"
)

// main delegates to run and exits with the resulting code.
func main() {
	os.Exit(run())
}

// run executes the root command and maps the outcome to a process exit code: a
// child process's own code is propagated verbatim via *exitcode.Error, a usage or
// validation error (rejected before the command ran) maps to exitcode.Usage, and
// any other failure maps to exitcode.Runtime. Child-process signal handling lives
// in the runner, so no root-level signal trapping is required here.
func run() int {
	root := cli.NewRootCmd(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})

	cmd, err := root.ExecuteC()
	if err == nil {
		return exitcode.OK
	}

	if exitErr, ok := errors.AsType[*exitcode.Error](err); ok {
		return exitErr.Code
	}

	fmt.Fprintln(os.Stderr, "Error:", err)

	// The root's PersistentPreRunE flips SilenceUsage on only after parsing and
	// validation succeed, so a command that still has it unset failed at the usage
	// boundary.
	if !cmd.SilenceUsage {
		return exitcode.Usage
	}
	return exitcode.Runtime
}
