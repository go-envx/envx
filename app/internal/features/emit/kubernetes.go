package emit

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// k8sIndent is the nesting indent for rendered Kubernetes manifests, matching the
// conventional two-space YAML style.
const k8sIndent = 2

// k8sMetadata is the minimal object metadata emit sets on a rendered resource.
type k8sMetadata struct {
	// Name is the resource's metadata.name.
	Name string `yaml:"name"`
}

// secretManifest is a Kubernetes v1 Secret whose data holds base64-encoded
// values, as the Secret schema requires.
type secretManifest struct {
	// APIVersion is the resource's schema version.
	APIVersion string `yaml:"apiVersion"`
	// Kind is the resource kind.
	Kind string `yaml:"kind"`
	// Metadata carries the resource name.
	Metadata k8sMetadata `yaml:"metadata"`
	// Type is the Secret's type; emit always renders an Opaque secret.
	Type string `yaml:"type"`
	// Data maps each key to its base64-encoded value.
	Data map[string]string `yaml:"data"`
}

// configMapManifest is a Kubernetes v1 ConfigMap whose data holds plaintext
// values.
type configMapManifest struct {
	// APIVersion is the resource's schema version.
	APIVersion string `yaml:"apiVersion"`
	// Kind is the resource kind.
	Kind string `yaml:"kind"`
	// Metadata carries the resource name.
	Metadata k8sMetadata `yaml:"metadata"`
	// Data maps each key to its plaintext value.
	Data map[string]string `yaml:"data"`
}

// buildSecretManifest assembles a v1 Secret whose data holds each entry's value
// base64-encoded with standard encoding, as the Secret schema requires.
func buildSecretManifest(name string, entries []Entry) secretManifest {
	data := make(map[string]string, len(entries))
	for _, e := range entries {
		data[e.Key] = base64.StdEncoding.EncodeToString([]byte(e.Value))
	}
	return secretManifest{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   k8sMetadata{Name: name},
		Type:       "Opaque",
		Data:       data,
	}
}

// buildConfigMapManifest assembles a v1 ConfigMap whose data holds each entry's
// plaintext value.
func buildConfigMapManifest(name string, entries []Entry) configMapManifest {
	data := make(map[string]string, len(entries))
	for _, e := range entries {
		data[e.Key] = e.Value
	}
	return configMapManifest{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   k8sMetadata{Name: name},
		Data:       data,
	}
}

// renderK8sSecret writes a v1 Secret carrying the given entries. An explicit
// single-kind target renders the resource even when empty, so the caller gets
// exactly the kind it asked for.
func renderK8sSecret(w io.Writer, name string, entries []Entry) error {
	return encodeYAML(w, buildSecretManifest(name, entries))
}

// renderK8sConfigMap writes a v1 ConfigMap carrying the given entries. Like the
// Secret target it renders even when empty, since the caller asked for this kind.
func renderK8sConfigMap(w io.Writer, name string, entries []Entry) error {
	return encodeYAML(w, buildConfigMapManifest(name, entries))
}

// renderK8sCombined writes both Kubernetes resources as a single multi-document
// YAML stream (`---`-separated) so one `kubectl apply -f` installs the whole
// environment. The ConfigMap and Secret take distinct names. A resource with no
// entries is skipped rather than emitted empty, so a secret-free environment
// ships no Secret and an all-secret one no ConfigMap; an entirely empty
// environment produces no output. The ConfigMap is written before the Secret for
// a stable document order.
func renderK8sCombined(
	w io.Writer, configMapName, secretName string, secrets, plain []Entry,
) error {
	// Nothing to render: emit no output rather than an empty document, which the
	// YAML encoder cannot close without a written stream.
	if len(plain) == 0 && len(secrets) == 0 {
		return nil
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(k8sIndent)
	if len(plain) > 0 {
		if err := enc.Encode(buildConfigMapManifest(configMapName, plain)); err != nil {
			_ = enc.Close()
			return err
		}
	}
	if len(secrets) > 0 {
		if err := enc.Encode(buildSecretManifest(secretName, secrets)); err != nil {
			_ = enc.Close()
			return err
		}
	}
	return enc.Close()
}

// renderK8sBundle packages the selected slice into a single resource data key
// (params.Key) rather than one key per value, so the resource can be
// volume-mounted as one file. The selection maps to a kind and a name: both
// slices merge into a Secret named base-config-secrets (it may carry secret
// material), a secrets-only bundle is a Secret named base-secrets, and a
// config-only bundle is a ConfigMap named base-config. The body under the key is
// the slice rendered in the format its extension names (.json or .env).
func renderK8sBundle(w io.Writer, sorted []Entry, params Params) error {
	if err := requireNameBase(params); err != nil {
		return err
	}
	format, err := parseBundleFormat(params.Key)
	if err != nil {
		return err
	}

	name := bundleResourceName(
		params.NameBase, params.ExactName, params.IncludeSecrets, params.IncludeConfig,
	)
	var (
		body   []Entry
		secret bool
	)
	switch {
	case params.IncludeSecrets && params.IncludeConfig:
		// A merged bundle may contain secrets, so it is always a Secret.
		body, secret = sorted, true
	case params.IncludeSecrets:
		body, secret = secretEntries(sorted, true), true
	default:
		body, secret = secretEntries(sorted, false), false
	}

	encoded, err := renderBundleBody(format, body)
	if err != nil {
		return err
	}

	// Reuse the manifest builders with a single synthetic entry: buildSecret
	// base64-encodes the value as the Secret schema requires, buildConfigMap keeps
	// it plaintext.
	entry := []Entry{{Key: params.Key, Value: encoded}}
	if secret {
		return encodeYAML(w, buildSecretManifest(name, entry))
	}
	return encodeYAML(w, buildConfigMapManifest(name, entry))
}

// bundleFormat is the on-disk format of a bundled body.
type bundleFormat int

const (
	// bundleJSON renders the body as a JSON object.
	bundleJSON bundleFormat = iota
	// bundleDotenv renders the body as KEY=value lines.
	bundleDotenv
)

// parseBundleFormat infers a bundle's body format from its key's extension, so
// bundle=config.json is JSON and bundle=app.env is dotenv. An unrecognized
// extension is rejected rather than guessed.
func parseBundleFormat(name string) (bundleFormat, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		return bundleJSON, nil
	case ".env":
		return bundleDotenv, nil
	default:
		return 0, fmt.Errorf(
			"cannot infer bundle format from %q (use a .json or .env extension)", name,
		)
	}
}

// renderBundleBody renders entries to a string in the chosen bundle format,
// reusing the ordinary single-document renderers.
func renderBundleBody(format bundleFormat, entries []Entry) (string, error) {
	var buffer bytes.Buffer
	var err error
	switch format {
	case bundleJSON:
		err = renderJSON(&buffer, entries)
	default:
		err = renderDotenv(&buffer, entries)
	}
	return buffer.String(), err
}

// encodeYAML marshals value as a YAML document with the shared indent, closing
// the encoder so the trailing document terminator and buffered bytes are flushed.
func encodeYAML(w io.Writer, value any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(k8sIndent)
	if err := enc.Encode(value); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}
