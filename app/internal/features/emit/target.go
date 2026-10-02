package emit

import (
	"fmt"
	"io"
	"strings"
)

// Target identifies the output shape emit renders an environment to. It captures
// both the format and the intended consumption: the two k8s targets differ in
// resource shape (many keys vs one bundled key), not just formatting. The slice
// of the environment each carries — secrets, config, or both — is chosen
// separately through RenderParams.
type Target string

const (
	// TargetK8s renders canonical multi-key Kubernetes resources: a v1 Secret for
	// the secret-derived values and a v1 ConfigMap for the plain ones, each value
	// its own data key. This is the shape a pod consumes with envFrom.
	TargetK8s Target = "k8s"
	// TargetK8sBundle renders the selected slice as a single data key holding one
	// JSON (or dotenv) blob, so the resource can be volume-mounted as one file.
	TargetK8sBundle Target = "k8s-bundle"
	// TargetJSON renders a flat JSON object of key/value pairs.
	TargetJSON Target = "json"
	// TargetDotenv renders KEY=value lines for a non-envx dotenv consumer.
	TargetDotenv Target = "dotenv"
)

// Targets lists every supported target in a stable order, for help text and flag
// validation messages.
func Targets() []Target {
	return []Target{TargetK8s, TargetK8sBundle, TargetJSON, TargetDotenv}
}

// Kubernetes reports whether the target renders a Kubernetes resource, which is
// exactly the set of targets that derive their names from the name base.
func (t Target) Kubernetes() bool {
	return t == TargetK8s || t == TargetK8sBundle
}

// ParseTarget converts a raw flag value into a Target, rejecting an unknown one
// with an ErrUnknownTarget naming the supported set.
func ParseTarget(value string) (Target, error) {
	for _, t := range Targets() {
		if Target(value) == t {
			return t, nil
		}
	}
	return "", unknownTargetError(value)
}

// unknownTargetError wraps ErrUnknownTarget with the rejected value and the
// supported set.
func unknownTargetError(value string) error {
	return fmt.Errorf("%w %q (want one of: %s)", ErrUnknownTarget, value, targetList())
}

// targetList renders the supported targets as a comma-separated string for error
// and help messages.
func targetList() string {
	names := make([]string, 0, len(Targets()))
	for _, t := range Targets() {
		names = append(names, string(t))
	}
	return strings.Join(names, ", ")
}

// Entry is one fully resolved environment variable to render. Value is
// plaintext; Secret marks a value the Kubernetes split routes into a Secret
// rather than a ConfigMap.
type Entry struct {
	// Key is the canonical env-var name.
	Key string
	// Value is the resolved plaintext value.
	Value string
	// Secret reports whether the value is secret-derived.
	Secret bool
}

// RenderParams configures one render request: the entries to serialize and how.
type RenderParams struct {
	// Entries are the resolved environment variables to render.
	Entries []Entry
	// Target selects the output shape.
	Target Target
	// NameBase is the base for a Kubernetes resource name. Unless ExactName is set,
	// the render appends the slice suffix (-config, -secrets, or -config-secrets).
	// Required for the k8s targets and ignored by the others.
	NameBase string
	// ExactName uses NameBase verbatim as the metadata name, without a slice
	// suffix — set when the caller supplied an explicit name to honor.
	ExactName bool
	// IncludeSecrets includes the secret-derived values in the output.
	IncludeSecrets bool
	// IncludeConfig includes the plain (non-secret) values in the output.
	IncludeConfig bool
	// Key is the single data key a k8s-bundle render packs the slice into; its
	// extension (.json or .env) selects the body format. Used by k8s-bundle only.
	Key string
	// Writer receives the rendered output; nil uses os.Stdout.
	Writer io.Writer
}
