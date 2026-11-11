package env

import (
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
)

// NamespaceData holds one parsed namespace file.
type NamespaceData struct {
	// Data is the unmarshaled key-value tree.
	Data map[string]any
	// SourcePath is the file the data was read from.
	SourcePath string
}

// NamespaceReader loads namespace base and overlay files.
type NamespaceReader interface {
	// LoadBase loads the base namespace tree for an include path.
	LoadBase(includePath string) (NamespaceData, error)
	// LoadOverlay loads an environment overlay, reporting false when absent.
	LoadOverlay(includePath, environment string) (NamespaceData, bool, error)
}

// OverlayWriter persists one key in an environment overlay.
type OverlayWriter interface {
	// SetOverlay writes text at the dotted key and returns the overlay path.
	SetOverlay(includePath, environment, key, text string) (string, error)
}

// ValueResolver dereferences and diagnoses one winning scalar value, returning
// unrecognized values unchanged.
type ValueResolver interface {
	// Resolve materializes raw under the resolver's reveal policy.
	Resolve(raw, environment string) (string, error)
	// Evaluate classifies and diagnoses raw without returning an error.
	Evaluate(raw, environment string) value.Evaluation
}

// ResolverParams configures one operation-scoped value resolver.
type ResolverParams struct {
	// Reveal decrypts or fetches values instead of masking them.
	Reveal bool
	// Host is the operation's captured host snapshot.
	Host hostenv.Snapshot
}

// Executor runs a child process.
type Executor func(process.Command) error

// Dependencies are the adapters a Service uses. Only EnvNamespaces is required;
// the others are required only by the operations that use them.
type Dependencies struct {
	// EnvNamespaces loads namespace files.
	EnvNamespaces NamespaceReader
	// EnvOverlays writes overlay values; required by Set.
	EnvOverlays OverlayWriter
	// HostEnv captures the host environment once per operation; nil is empty.
	HostEnv func() hostenv.Snapshot
	// NewValueResolver creates an operation-scoped resolver; nil is identity
	// resolution.
	NewValueResolver func(params ResolverParams) (ValueResolver, error)
	// ProcessRunner runs child processes; required by Run.
	ProcessRunner Executor
}
