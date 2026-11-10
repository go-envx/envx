// Command envx is the entry point for the envx CLI. It wires the root command to
// the process streams and exits with the code cli.Execute reports.
package main

import (
	"os"

	"github.com/go-envx/envx/app/internal/app"
	"github.com/go-envx/envx/app/internal/cli"
	"github.com/go-envx/envx/app/internal/lib/stdio"
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

// main builds the root command and exits with its outcome. Child-process signal
// handling lives in lib/process, so no root-level signal trapping is needed.
func main() {
	root := cli.NewRootCommand(
		cli.BuildInfo{
			Version: version,
			Commit:  commit,
			Date:    date,
		},
		app.Factory{},
		stdio.Streams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	)
	os.Exit(cli.Execute(root))
}
