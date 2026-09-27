package runner

// RunParams specifies the child command to spawn and its injected environment.
type RunParams struct {
	Args []string
	Env  map[string]string
}
