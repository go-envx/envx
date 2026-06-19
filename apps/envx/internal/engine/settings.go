package engine

// -------------------------------------------------------------------------------------
// Flags holds the env-resolution settings the engine consumes — a plain data
// struct with no knowledge of cobra. Actions bind cobra flags into a value of
// this type at the edge (see internal/actions.RegisterEngineFlags) and hand it
// to the engine via Request.Flags, keeping the core framework-free.
type Flags struct {
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}
