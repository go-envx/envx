package cli

import (
	"fmt"
	"runtime"
)

// -------------------------------------------------------------------------------------

// BuildInfo carries the build metadata injected at link time (see main's
// -ldflags). It is passed to NewRootCmd and rendered by the --version flag so a
// released binary can be traced back to the exact revision it was built from.
type BuildInfo struct {
	// Version is the semantic version or VCS tag of the build.
	Version string
	// Commit is the VCS revision the binary was built from.
	Commit string
	// Date is the build timestamp.
	Date string
}

// -------------------------------------------------------------------------------------

// formatVersion renders BuildInfo into the string Cobra prints for --version,
// appending the Go toolchain version and target platform so a bug report carries
// the full build context.
func formatVersion(info BuildInfo) string {
	return fmt.Sprintf(
		"%s\ncommit: %s\nbuilt:  %s\n%s %s/%s",
		info.Version, info.Commit, info.Date,
		runtime.Version(), runtime.GOOS, runtime.GOARCH,
	)
}
