package engine

// -------------------------------------------------------------------------------------
// Settings holds the fully-resolved env-resolution settings the engine consumes
// a plain data struct with no knowledge of external callers or config precedence.
type Settings struct {
	Env             string
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}
