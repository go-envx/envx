package runner

import "io"

// RunParams specifies the child command to spawn, its injected environment, and
// its standard streams.
type RunParams struct {
	// Args is the child command followed by its arguments.
	Args []string
	// Env is the complete environment injected into the child.
	Env map[string]string
	// Stdout receives the child's standard output; nil uses os.Stdout.
	Stdout io.Writer
	// Stderr receives the child's standard error; nil uses os.Stderr.
	Stderr io.Writer
	// Stdin supplies the child's standard input; nil uses os.Stdin.
	Stdin io.Reader
}
