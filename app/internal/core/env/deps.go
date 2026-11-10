package env

import (
	"io"

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

// Executor runs a child process.
type Executor func(process.Command) error

// Renderer serializes resolved entries to an emit target.
type Renderer interface {
	// Validate rejects unsupported targets and option combinations.
	Validate(options EmitOptions) error
	// Render writes entries for project in the target's format.
	Render(
		out io.Writer, project string, entries []ExplainEntry, options EmitOptions,
	) error
}

// Dependencies are the adapters a Service uses. Only EnvNamespaces is required;
// the others are required only by the operations that use them.
type Dependencies struct {
	// EnvNamespaces loads namespace files.
	EnvNamespaces NamespaceReader
	// EnvOverlays writes overlay values; required by Set.
	EnvOverlays OverlayWriter
	// HostEnv captures the host environment once per operation; nil is empty.
	HostEnv func() hostenv.Snapshot
	// NewValueResolver creates an operation-scoped resolver bound to the
	// captured host snapshot; nil is identity resolution.
	NewValueResolver func(reveal bool, host hostenv.Snapshot) (ValueResolver, error)
	// ProcessRunner runs child processes; required by Run.
	ProcessRunner Executor
	// FormatRenderer serializes emit output; required by Emit.
	FormatRenderer Renderer
}
