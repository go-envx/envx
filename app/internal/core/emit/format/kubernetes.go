package format

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// k8sIndent is the YAML indentation for rendered manifests.
const k8sIndent = 2

// Bundle body formats inferred from the data key extension.
const (
	// bundleJSON renders the body as a JSON object.
	bundleJSON bundleFormat = iota
	// bundleDotenv renders the body as KEY=value lines.
	bundleDotenv
)

// bundleFormat is a bundled body's format.
type bundleFormat int

// k8sMetadata is the resource metadata emit sets.
type k8sMetadata struct {
	// Name is metadata.name.
	Name string `yaml:"name"`
}

// secretManifest is a v1 Secret with base64-encoded data.
type secretManifest struct {
	// APIVersion is the schema version.
	APIVersion string `yaml:"apiVersion"`
	// Kind is the resource kind.
	Kind string `yaml:"kind"`
	// Metadata carries the name.
	Metadata k8sMetadata `yaml:"metadata"`
	// Type is always Opaque.
	Type string `yaml:"type"`
	// Data maps keys to base64-encoded values.
	Data map[string]string `yaml:"data"`
}

// configMapManifest is a v1 ConfigMap with plaintext data.
type configMapManifest struct {
	// APIVersion is the schema version.
	APIVersion string `yaml:"apiVersion"`
	// Kind is the resource kind.
	Kind string `yaml:"kind"`
	// Metadata carries the name.
	Metadata k8sMetadata `yaml:"metadata"`
	// Data maps keys to plaintext values.
	Data map[string]string `yaml:"data"`
}

// config returns the ConfigMap name.
func (n naming) config() string { return n.suffixed("-config") }

// secrets returns the Secret name.
func (n naming) secrets() string { return n.suffixed("-secrets") }

// merged returns the merged bundle name.
func (n naming) merged() string { return n.suffixed("-config-secrets") }

// suffixed returns base verbatim when exact, else base+suffix.
func (n naming) suffixed(suffix string) string {
	if n.exact {
		return n.base
	}
	return n.base + suffix
}

// renderK8s renders one resource per value kind. Selecting both slices emits a
// ConfigMap then a Secret, skipping an empty one; a single slice always renders
// its resource.
func renderK8s(w io.Writer, items []item, names naming, selected selection) error {
	secrets := partition(items, true)
	config := partition(items, false)
	switch {
	case selected.secrets && selected.config:
		return renderK8sCombined(w, names, secrets, config)
	case selected.secrets:
		return encodeYAML(w, buildSecret(names.secrets(), secrets))
	default:
		return encodeYAML(w, buildConfigMap(names.config(), config))
	}
}

// renderK8sCombined writes non-empty resources as one multi-document stream;
// an empty environment produces no output.
func renderK8sCombined(w io.Writer, names naming, secrets, config []item) error {
	if len(config) == 0 && len(secrets) == 0 {
		return nil
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(k8sIndent)
	if len(config) > 0 {
		if err := enc.Encode(buildConfigMap(names.config(), config)); err != nil {
			_ = enc.Close()
			return err
		}
	}
	if len(secrets) > 0 {
		if err := enc.Encode(buildSecret(names.secrets(), secrets)); err != nil {
			_ = enc.Close()
			return err
		}
	}
	return enc.Close()
}

// renderK8sBundle packs the selected slice into one data key whose extension
// selects the body format. Any selection containing secrets is a Secret.
func renderK8sBundle(
	w io.Writer, items []item, names naming, key string, selected selection,
) error {
	format, err := parseBundleFormat(key)
	if err != nil {
		return err
	}
	var (
		name   string
		body   []item
		secret bool
	)
	switch {
	case selected.secrets && selected.config:
		name, body, secret = names.merged(), items, true
	case selected.secrets:
		name, body, secret = names.secrets(), partition(items, true), true
	default:
		name, body, secret = names.config(), partition(items, false), false
	}

	var buffer bytes.Buffer
	if format == bundleJSON {
		err = renderJSON(&buffer, body)
	} else {
		err = renderDotenv(&buffer, body)
	}
	if err != nil {
		return err
	}

	entry := []item{{key: key, value: buffer.String()}}
	if secret {
		return encodeYAML(w, buildSecret(name, entry))
	}
	return encodeYAML(w, buildConfigMap(name, entry))
}

// parseBundleFormat infers the body format from key's extension.
func parseBundleFormat(key string) (bundleFormat, error) {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".json":
		return bundleJSON, nil
	case ".env":
		return bundleDotenv, nil
	default:
		return 0, fmt.Errorf(
			"%w: cannot infer bundle format from %q (use a .json or .env extension)",
			ErrInvalidOption, key,
		)
	}
}

// buildSecret assembles a v1 Secret with base64-encoded values.
func buildSecret(name string, items []item) secretManifest {
	data := make(map[string]string, len(items))
	for _, it := range items {
		data[it.key] = base64.StdEncoding.EncodeToString([]byte(it.value))
	}
	return secretManifest{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   k8sMetadata{Name: name},
		Type:       "Opaque",
		Data:       data,
	}
}

// buildConfigMap assembles a v1 ConfigMap with plaintext values.
func buildConfigMap(name string, items []item) configMapManifest {
	data := make(map[string]string, len(items))
	for _, it := range items {
		data[it.key] = it.value
	}
	return configMapManifest{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   k8sMetadata{Name: name},
		Data:       data,
	}
}

// encodeYAML writes one YAML document and flushes the encoder.
func encodeYAML(w io.Writer, value any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(k8sIndent)
	if err := enc.Encode(value); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}
