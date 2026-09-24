package emit

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// render is a test helper that renders entries to a string, failing the test on
// any render error so each case asserts only on output. When a case selects
// neither slice it defaults to both, mirroring the command's "emit everything"
// default so the all-values cases stay terse.
func render(t *testing.T, entries []Entry, params Params) string {
	t.Helper()
	if !params.IncludeSecrets && !params.IncludeConfig {
		params.IncludeSecrets, params.IncludeConfig = true, true
	}
	var buffer bytes.Buffer
	if err := Render(&buffer, entries, params); err != nil {
		t.Fatalf("Render(%s): %v", params.Target, err)
	}
	return buffer.String()
}

// sampleEntries is a mixed set of secret-derived and plain values in unsorted
// order, so tests can assert both the secret/plain split and deterministic
// sorting.
func sampleEntries() []Entry {
	return []Entry{
		{Key: "PORT", Value: "5432", Secret: false},
		{Key: "DB_PASSWORD", Value: "s3cr3t", Secret: true},
		{Key: "APP_NAME", Value: "api", Secret: false},
		{Key: "API_KEY", Value: "abc123", Secret: true},
	}
}

// TestRenderDotenv verifies dotenv output lists every entry as KEY=value in
// sorted key order regardless of the secret flag.
func TestRenderDotenv(t *testing.T) {
	t.Parallel()

	got := render(t, sampleEntries(), Params{Target: TargetDotenv})
	want := "API_KEY=abc123\n" +
		"APP_NAME=api\n" +
		"DB_PASSWORD=s3cr3t\n" +
		"PORT=5432\n"
	if got != want {
		t.Errorf("dotenv = %q, want %q", got, want)
	}
}

// TestRenderDotenvQuoting verifies a value with a special character is quoted and
// escaped while a plain value is written bare, and an empty value stays explicit.
func TestRenderDotenvQuoting(t *testing.T) {
	t.Parallel()

	entries := []Entry{
		{Key: "PLAIN", Value: "http://host:8080/path"},
		{Key: "SPACED", Value: "two words"},
		{Key: "QUOTED", Value: `a "b" c`},
		{Key: "NEWLINE", Value: "line1\nline2"},
		{Key: "EMPTY", Value: ""},
	}
	got := render(t, entries, Params{Target: TargetDotenv})
	want := "EMPTY=\"\"\n" +
		"NEWLINE=\"line1\\nline2\"\n" +
		"PLAIN=http://host:8080/path\n" +
		"QUOTED=\"a \\\"b\\\" c\"\n" +
		"SPACED=\"two words\"\n"
	if got != want {
		t.Errorf("dotenv = %q, want %q", got, want)
	}
}

// TestRenderJSON verifies JSON output is a sorted, indented object of every
// entry's key and plaintext value.
func TestRenderJSON(t *testing.T) {
	t.Parallel()

	got := render(t, sampleEntries(), Params{Target: TargetJSON})
	want := "{\n" +
		"  \"API_KEY\": \"abc123\",\n" +
		"  \"APP_NAME\": \"api\",\n" +
		"  \"DB_PASSWORD\": \"s3cr3t\",\n" +
		"  \"PORT\": \"5432\"\n" +
		"}\n"
	if got != want {
		t.Errorf("json = %q, want %q", got, want)
	}
}

// TestRenderK8sSecret verifies the k8s target with only the secret slice selected
// renders a valid v1 Secret that carries only the secret-derived values, each
// base64-encoded, and omits the plain ones.
func TestRenderK8sSecret(t *testing.T) {
	t.Parallel()

	got := render(t, sampleEntries(), Params{
		Target: TargetK8s, NameBase: "app", IncludeSecrets: true,
	})
	for _, line := range []string{
		"apiVersion: v1",
		"kind: Secret",
		"  name: app-secrets",
		"type: Opaque",
		"  API_KEY: " + base64.StdEncoding.EncodeToString([]byte("abc123")),
		"  DB_PASSWORD: " + base64.StdEncoding.EncodeToString([]byte("s3cr3t")),
	} {
		if !strings.Contains(got, line) {
			t.Errorf("secret manifest missing %q\n%s", line, got)
		}
	}
	// Plain values must not leak into the Secret.
	for _, plain := range []string{"PORT", "APP_NAME"} {
		if strings.Contains(got, plain) {
			t.Errorf("secret manifest unexpectedly contains plain key %q\n%s", plain, got)
		}
	}
}

// TestRenderK8sConfigMap verifies the k8s target with only the config slice
// selected renders a valid v1 ConfigMap that carries only the plain values as
// plaintext and omits the secret-derived ones.
func TestRenderK8sConfigMap(t *testing.T) {
	t.Parallel()

	params := Params{Target: TargetK8s, NameBase: "app", IncludeConfig: true}
	got := render(t, sampleEntries(), params)
	for _, line := range []string{
		"apiVersion: v1",
		"kind: ConfigMap",
		"  name: app-config",
		"  APP_NAME: api",
		"  PORT: \"5432\"",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("configmap missing %q\n%s", line, got)
		}
	}
	// Secret-derived values must not leak into the ConfigMap.
	for _, secret := range []string{"DB_PASSWORD", "API_KEY", "s3cr3t", "abc123"} {
		if strings.Contains(got, secret) {
			t.Errorf("configmap unexpectedly contains secret material %q\n%s", secret, got)
		}
	}
}

// TestRenderK8sCombined verifies the combined k8s target emits both resources as
// one multi-document manifest, with the ConfigMap first and the Secret second,
// each with its slice-suffixed name and only its own kind's values.
func TestRenderK8sCombined(t *testing.T) {
	t.Parallel()

	got := render(t, sampleEntries(), Params{Target: TargetK8s, NameBase: "api"})
	// Both documents, separated by a YAML document marker, ConfigMap before Secret.
	if !strings.Contains(got, "kind: ConfigMap") ||
		!strings.Contains(got, "kind: Secret") {
		t.Errorf("combined manifest should carry both kinds\n%s", got)
	}
	// The two resources take distinct, slice-suffixed names.
	if !strings.Contains(got, "  name: api-config") ||
		!strings.Contains(got, "  name: api-secrets") {
		t.Errorf("combined manifest should name resources api-config/api-secrets\n%s", got)
	}
	if !strings.Contains(got, "\n---\n") {
		t.Errorf("combined manifest should separate documents with ---\n%s", got)
	}
	if strings.Index(got, "kind: ConfigMap") > strings.Index(got, "kind: Secret") {
		t.Errorf("ConfigMap should precede Secret\n%s", got)
	}
	// The Secret keeps its values base64-encoded; the ConfigMap stays plaintext.
	wantSecret := "  DB_PASSWORD: " + base64.StdEncoding.EncodeToString([]byte("s3cr3t"))
	if !strings.Contains(got, wantSecret) {
		t.Errorf("combined manifest missing base64 secret value\n%s", got)
	}
	if !strings.Contains(got, "  APP_NAME: api") {
		t.Errorf("combined manifest missing plain configmap value\n%s", got)
	}
}

// TestRenderK8sCombinedSkipsEmpty verifies the combined target omits a resource
// with no entries rather than emitting it empty: an all-plain environment yields
// only a ConfigMap, an all-secret one only a Secret, and an empty one nothing.
func TestRenderK8sCombinedSkipsEmpty(t *testing.T) {
	t.Parallel()

	onlyPlain := []Entry{{Key: "APP_NAME", Value: "api", Secret: false}}
	got := render(t, onlyPlain, Params{Target: TargetK8s, NameBase: "api"})
	if !strings.Contains(got, "kind: ConfigMap") || strings.Contains(got, "kind: Secret") {
		t.Errorf("all-plain environment should emit only a ConfigMap\n%s", got)
	}

	onlySecret := []Entry{{Key: "API_KEY", Value: "abc123", Secret: true}}
	got = render(t, onlySecret, Params{Target: TargetK8s, NameBase: "api"})
	if !strings.Contains(got, "kind: Secret") || strings.Contains(got, "kind: ConfigMap") {
		t.Errorf("all-secret environment should emit only a Secret\n%s", got)
	}

	if got := render(t, nil, Params{Target: TargetK8s, NameBase: "api"}); got != "" {
		t.Errorf("empty environment should emit nothing for the combined target, got %q", got)
	}
}

// TestRenderK8sRequiresName verifies the k8s target rejects an empty resource
// name for each slice selection, since a valid manifest needs metadata.name.
func TestRenderK8sRequiresName(t *testing.T) {
	t.Parallel()

	slices := []Params{
		{IncludeSecrets: true, IncludeConfig: true},
		{IncludeSecrets: true},
		{IncludeConfig: true},
	}
	for _, sel := range slices {
		sel.Target = TargetK8s
		var buffer bytes.Buffer
		err := Render(&buffer, sampleEntries(), sel)
		if err == nil {
			t.Errorf("Render(k8s, %+v) without a name should fail", sel)
		}
		if buffer.Len() != 0 {
			t.Errorf("Render(k8s) wrote output despite the missing name: %q", buffer.String())
		}
	}
}

// TestRenderK8sBundle verifies the k8s-bundle target packs the selected slice
// into a single resource data key whose body is the extension's format, and names
// the resource by slice: a config-only .json bundle is a ConfigMap (base-config)
// with a plaintext JSON blob, a secrets-only .env bundle is a Secret
// (base-secrets) with a base64 dotenv blob, and an unrestricted bundle merges
// everything into one Secret (base-config-secrets).
func TestRenderK8sBundle(t *testing.T) {
	t.Parallel()

	// config-only, JSON body, ConfigMap (plaintext), named api-config.
	cm := render(t, sampleEntries(), Params{
		Target: TargetK8sBundle, NameBase: "api", IncludeConfig: true, Key: "config.json",
	})
	if !strings.Contains(cm, "kind: ConfigMap") ||
		!strings.Contains(cm, "  name: api-config") ||
		!strings.Contains(cm, "config.json: |") {
		t.Errorf("config bundle should be ConfigMap api-config, key config.json\n%s", cm)
	}
	if !strings.Contains(cm, `"APP_NAME": "api"`) || strings.Contains(cm, "DB_PASSWORD") {
		t.Errorf("config bundle body should hold only plain values as JSON\n%s", cm)
	}

	// secrets-only, dotenv body, Secret (base64), named api-secrets.
	sec := render(t, sampleEntries(), Params{
		Target: TargetK8sBundle, NameBase: "api", IncludeSecrets: true, Key: "secrets.env",
	})
	if !strings.Contains(sec, "kind: Secret") ||
		!strings.Contains(sec, "  name: api-secrets") ||
		!strings.Contains(sec, "secrets.env:") {
		t.Errorf("secret bundle should be Secret api-secrets with a secrets.env key\n%s", sec)
	}
	wantBody := "API_KEY=abc123\nDB_PASSWORD=s3cr3t\n"
	if !strings.Contains(sec, base64.StdEncoding.EncodeToString([]byte(wantBody))) {
		t.Errorf("secret bundle body should be the base64 dotenv of the secrets\n%s", sec)
	}

	// unrestricted → merged into one Secret named api-config-secrets.
	merged := render(t, sampleEntries(), Params{
		Target: TargetK8sBundle, NameBase: "api",
		IncludeSecrets: true, IncludeConfig: true, Key: "app.json",
	})
	if !strings.Contains(merged, "kind: Secret") ||
		!strings.Contains(merged, "  name: api-config-secrets") ||
		strings.Contains(merged, "kind: ConfigMap") {
		t.Errorf("merged bundle should be a single Secret api-config-secrets\n%s", merged)
	}
}

// TestRenderK8sBundleBadExtension verifies a bundle key whose extension names no
// known format is rejected and writes nothing.
func TestRenderK8sBundleBadExtension(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := Render(&buffer, sampleEntries(), Params{
		Target: TargetK8sBundle, NameBase: "api", IncludeConfig: true, Key: "config.yaml",
	})
	if err == nil {
		t.Error("a bundle with an unknown extension should fail")
	}
	if buffer.Len() != 0 {
		t.Errorf("bundle error wrote output: %q", buffer.String())
	}
}

// TestRenderRequiresASlice verifies Render refuses to emit when neither slice is
// selected, since there would be nothing to render.
func TestRenderRequiresASlice(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	if err := Render(&buffer, sampleEntries(), Params{Target: TargetJSON}); err == nil {
		t.Error("Render with neither slice selected should fail")
	}
	if buffer.Len() != 0 {
		t.Errorf("Render wrote output despite no slice selection: %q", buffer.String())
	}
}

// TestRenderUnknownTarget verifies an unrecognized target is rejected and writes
// nothing.
func TestRenderUnknownTarget(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	if err := Render(&buffer, sampleEntries(), Params{Target: "toml"}); err == nil {
		t.Error("Render with an unknown target should fail")
	}
	if buffer.Len() != 0 {
		t.Errorf("Render wrote output for an unknown target: %q", buffer.String())
	}
}

// TestParseTarget verifies every supported target parses and an unknown one is
// rejected with a message naming the value.
func TestParseTarget(t *testing.T) {
	t.Parallel()

	for _, target := range Targets() {
		got, err := ParseTarget(string(target))
		if err != nil {
			t.Errorf("ParseTarget(%q): %v", target, err)
		}
		if got != target {
			t.Errorf("ParseTarget(%q) = %q, want %q", target, got, target)
		}
	}
	if _, err := ParseTarget("yaml"); err == nil {
		t.Error("ParseTarget(\"yaml\") should fail")
	} else if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("error %q should name the rejected value", err)
	}
}

// TestRenderEmptyEnvironment verifies an environment with no entries still
// produces valid, non-partial output for each target.
func TestRenderEmptyEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target Target
		want   string
	}{
		{TargetDotenv, ""},
		{TargetJSON, "{}\n"},
	}
	for _, tt := range tests {
		if got := render(t, nil, Params{Target: tt.target}); got != tt.want {
			t.Errorf("Render(%s) empty = %q, want %q", tt.target, got, tt.want)
		}
	}
	// An explicitly-selected single slice renders its resource even when empty,
	// since the caller asked for that kind.
	got := render(t, nil, Params{
		Target: TargetK8s, NameBase: "empty", IncludeSecrets: true,
	})
	if !strings.Contains(got, "kind: Secret") || !strings.Contains(got, "data: {}") {
		t.Errorf("empty secret slice should render an empty Secret, got:\n%s", got)
	}
}
