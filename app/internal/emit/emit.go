package emit

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Target identifies the output shape emit renders an environment to. It captures
// both the format and the intended consumption: the two k8s targets differ in
// resource shape (many keys vs one bundled key), not just formatting. The slice
// of the environment each carries — secrets, config, or both — is chosen
// separately through Params.
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
// with a message naming the supported set.
func ParseTarget(value string) (Target, error) {
	for _, t := range Targets() {
		if Target(value) == t {
			return t, nil
		}
	}
	return "", fmt.Errorf("unknown target %q (want one of: %s)", value, targetList())
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

// Params configures a render beyond the entries themselves.
type Params struct {
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
}

// Render writes entries to w in the target's shape, restricted to the selected
// slice: IncludeSecrets keeps the secret-derived values and IncludeConfig the
// plain ones (at least one must be set). json and dotenv render the selected
// values as a single document. k8s renders a Secret for the secrets and a
// ConfigMap for the config, each value its own key; k8s-bundle renders the slice
// as a single Key holding one blob. Entries render in sorted key order so output
// is deterministic.
func Render(w io.Writer, entries []Entry, params Params) error {
	if !params.IncludeSecrets && !params.IncludeConfig {
		return fmt.Errorf("nothing to render: select secrets, config, or both")
	}
	sorted := sortedEntries(entries)
	switch params.Target {
	case TargetJSON:
		return renderJSON(w, sliceEntries(sorted, params))
	case TargetDotenv:
		return renderDotenv(w, sliceEntries(sorted, params))
	case TargetK8s:
		return renderK8s(w, sorted, params)
	case TargetK8sBundle:
		return renderK8sBundle(w, sorted, params)
	default:
		return fmt.Errorf("unknown target %q (want one of: %s)", params.Target, targetList())
	}
}

// renderK8s renders the canonical multi-key resources for the selected slice.
// Selecting both emits a ConfigMap and a Secret (named base-config and
// base-secrets) as one multi-document manifest, skipping whichever is empty; a
// single slice emits just that resource.
func renderK8s(w io.Writer, sorted []Entry, params Params) error {
	if err := requireNameBase(params); err != nil {
		return err
	}
	secrets := secretEntries(sorted, true)
	config := secretEntries(sorted, false)
	cmName := configName(params.NameBase, params.ExactName)
	secName := secretName(params.NameBase, params.ExactName)
	switch {
	case params.IncludeSecrets && params.IncludeConfig:
		return renderK8sCombined(w, cmName, secName, secrets, config)
	case params.IncludeSecrets:
		return renderK8sSecret(w, secName, secrets)
	default:
		return renderK8sConfigMap(w, cmName, config)
	}
}

// sliceEntries returns the entries the selected slice keeps: secret-derived
// values when IncludeSecrets is set and plain values when IncludeConfig is set.
// It is used by the single-document targets (json, dotenv); the k8s targets split
// by resource instead.
func sliceEntries(entries []Entry, params Params) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if (e.Secret && params.IncludeSecrets) || (!e.Secret && params.IncludeConfig) {
			out = append(out, e)
		}
	}
	return out
}

// requireNameBase reports an error when a Kubernetes target was given no name
// base, since a valid resource cannot be rendered without a metadata name.
func requireNameBase(params Params) error {
	if params.NameBase == "" {
		return fmt.Errorf("target %q requires a resource name base", params.Target)
	}
	return nil
}

// configName, secretName, and mergedName derive a resource's metadata name from
// the base. With exact set the base is used verbatim (an explicit --name the
// caller wants honored); otherwise the slice suffix is appended, so a
// project-derived base yields the conventional distinct names (app-config,
// app-secrets).
func configName(base string, exact bool) string {
	return suffixed(base, exact, "-config")
}

func secretName(base string, exact bool) string {
	return suffixed(base, exact, "-secrets")
}

func mergedName(base string, exact bool) string {
	return suffixed(base, exact, "-config-secrets")
}

// suffixed returns base verbatim when exact, else base+suffix.
func suffixed(base string, exact bool, suffix string) string {
	if exact {
		return base
	}
	return base + suffix
}

// bundleResourceName returns the single resource name a k8s-bundle render uses
// for the given base and slice selection.
func bundleResourceName(base string, exact, includeSecrets, includeConfig bool) string {
	switch {
	case includeSecrets && includeConfig:
		return mergedName(base, exact)
	case includeSecrets:
		return secretName(base, exact)
	default:
		return configName(base, exact)
	}
}

// ValidateBundleKey reports whether key names a supported bundle body format via
// its extension (.json or .env), for early validation before any resolution.
func ValidateBundleKey(key string) error {
	_, err := parseBundleFormat(key)
	return err
}

// secretEntries returns the entries whose Secret flag equals want, preserving
// order, so the Kubernetes split routes each value to exactly one resource.
func secretEntries(entries []Entry, want bool) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Secret == want {
			out = append(out, e)
		}
	}
	return out
}

// sortedEntries returns a copy of entries sorted by key so every target renders
// deterministically regardless of the caller's ordering.
func sortedEntries(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	copy(out, entries)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
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
