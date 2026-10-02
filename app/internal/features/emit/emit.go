package emit

import (
	"fmt"
	"io"
	"sort"
)

// renderK8s renders the canonical multi-key resources for the selected slice.
// Selecting both emits a ConfigMap and a Secret (named base-config and
// base-secrets) as one multi-document manifest, skipping whichever is empty; a
// single slice emits just that resource.
func renderK8s(w io.Writer, sorted []Entry, params RenderParams) error {
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
func sliceEntries(entries []Entry, params RenderParams) []Entry {
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
func requireNameBase(params RenderParams) error {
	if params.NameBase == "" {
		return fmt.Errorf("%w: target %q", ErrMissingNameBase, params.Target)
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
