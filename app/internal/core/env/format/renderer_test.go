package format_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/env/format"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/lib/filex"
)

// entry builds a resolved entry.
func entry(key, text string, secret bool) env.ExplainEntry {
	return env.ExplainEntry{
		Key:        key,
		Evaluation: value.Evaluation{Value: text, Resolved: true},
		Secret:     secret,
	}
}

// sampleEntries mixes secret and plain values in unsorted order.
func sampleEntries() []env.ExplainEntry {
	return []env.ExplainEntry{
		entry("PORT", "5432", false),
		entry("DB_PASSWORD", "s3cr3t", true),
		entry("APP_NAME", "api", false),
		entry("API_KEY", "abc123", true),
	}
}

// render renders entries for project "api", failing the test on error.
func render(t *testing.T, entries []env.ExplainEntry, options env.EmitOptions) string {
	t.Helper()
	var buffer bytes.Buffer
	if err := (format.Renderer{}).Render(&buffer, "api", entries, options); err != nil {
		t.Fatalf("Render(%+v): %v", options, err)
	}
	return buffer.String()
}

// TestRenderGolden verifies each target against reviewed expected output.
func TestRenderGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// golden names the expected output file.
		golden string
		// options configures rendering.
		options env.EmitOptions
	}{
		{golden: "all.json", options: env.EmitOptions{Target: env.TargetJSON}},
		{golden: "all.dotenv", options: env.EmitOptions{Target: env.TargetDotenv}},
		{
			golden:  "config.dotenv",
			options: env.EmitOptions{Target: env.TargetDotenv, Only: env.OnlyConfig},
		},
		{golden: "k8s.yaml", options: env.EmitOptions{Target: env.TargetKubernetes}},
		{
			golden: "k8s-secrets-named.yaml",
			options: env.EmitOptions{
				Target: env.TargetKubernetes, Only: env.OnlySecrets, Name: "web",
			},
		},
		{
			golden:  "k8s-bundle.yaml",
			options: env.EmitOptions{Target: env.TargetKubernetesBundle},
		},
		{
			golden: "k8s-bundle-config.yaml",
			options: env.EmitOptions{
				Target: env.TargetKubernetesBundle, Only: env.OnlyConfig, Key: "app.env",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			want, err := filex.Read(filepath.Join("testdata", tt.golden))
			if err != nil {
				t.Fatalf("reading golden: %v", err)
			}
			if got := render(t, sampleEntries(), tt.options); got != string(want) {
				t.Errorf("output mismatch\n--- got\n%s--- want\n%s", got, want)
			}
		})
	}
}

// TestRenderDotenvQuoting verifies quoting and escaping of special values.
func TestRenderDotenvQuoting(t *testing.T) {
	t.Parallel()

	got := render(t, []env.ExplainEntry{
		entry("PLAIN", "http://host:8080/path", false),
		entry("SPACED", "two words", false),
		entry("QUOTED", `a "b" c`, false),
		entry("NEWLINE", "line1\nline2", false),
		entry("EMPTY", "", false),
		entry("HTML", "<a>&", false),
		entry("DOLLAR", "pa$word ${HOME}", false),
		entry("DOLLAR_QUOTE", "it's $HOME", false),
	}, env.EmitOptions{Target: env.TargetDotenv})
	want := "DOLLAR='pa$word ${HOME}'\n" +
		"DOLLAR_QUOTE=\"it's \\$HOME\"\n" +
		"EMPTY=''\n" +
		"HTML=<a>&\n" +
		"NEWLINE=\"line1\\nline2\"\n" +
		"PLAIN=http://host:8080/path\n" +
		"QUOTED='a \"b\" c'\n" +
		"SPACED='two words'\n"
	if got != want {
		t.Errorf("dotenv = %q, want %q", got, want)
	}
}

// TestRenderSecretBundleKey verifies a secrets-only bundle defaults its key
// and encodes the body.
func TestRenderSecretBundleKey(t *testing.T) {
	t.Parallel()

	got := render(t, sampleEntries(), env.EmitOptions{
		Target: env.TargetKubernetesBundle, Only: env.OnlySecrets,
	})
	body := "{\n  \"API_KEY\": \"abc123\",\n  \"DB_PASSWORD\": \"s3cr3t\"\n}\n"
	want := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: api-secrets\n" +
		"type: Opaque\ndata:\n  secrets.json: " +
		base64.StdEncoding.EncodeToString([]byte(body)) + "\n"
	if got != want {
		t.Errorf("bundle = %q, want %q", got, want)
	}
}

// TestRenderEmpty verifies empty environments render valid output.
func TestRenderEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// options configures rendering.
		options env.EmitOptions
		// want is the expected output.
		want string
	}{
		{options: env.EmitOptions{Target: env.TargetDotenv}},
		{options: env.EmitOptions{Target: env.TargetJSON}, want: "{}\n"},
		{options: env.EmitOptions{Target: env.TargetKubernetes}},
		{
			options: env.EmitOptions{Target: env.TargetKubernetes, Only: env.OnlySecrets},
			want: "apiVersion: v1\nkind: Secret\nmetadata:\n  name: api-secrets\n" +
				"type: Opaque\ndata: {}\n",
		},
	}
	for _, tt := range tests {
		if got := render(t, nil, tt.options); got != tt.want {
			t.Errorf("Render(%+v) = %q, want %q", tt.options, got, tt.want)
		}
	}
}

// TestValidate verifies unsupported targets and option combinations fail.
func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// options are validated.
		options env.EmitOptions
		// want is the expected error.
		want error
	}{
		{options: env.EmitOptions{Target: env.TargetJSON, Only: env.OnlyAll}},
		{options: env.EmitOptions{Target: env.TargetKubernetes, Name: "web"}},
		{options: env.EmitOptions{Target: env.TargetKubernetesBundle, Key: "a.ENV"}},
		{options: env.EmitOptions{Target: "toml"}, want: format.ErrUnknownTarget},
		{
			options: env.EmitOptions{Target: env.TargetJSON, Only: "both"},
			want:    format.ErrInvalidOption,
		},
		{
			options: env.EmitOptions{Target: env.TargetDotenv, Name: "web"},
			want:    format.ErrInvalidOption,
		},
		{
			options: env.EmitOptions{Target: env.TargetKubernetes, Key: "a.json"},
			want:    format.ErrInvalidOption,
		},
		{
			options: env.EmitOptions{Target: env.TargetKubernetesBundle, Key: "a.yaml"},
			want:    format.ErrInvalidOption,
		},
	}
	for _, tt := range tests {
		err := (format.Renderer{}).Validate(tt.options)
		if !errors.Is(err, tt.want) {
			t.Errorf("Validate(%+v) = %v, want %v", tt.options, err, tt.want)
		}
	}
}

// TestRenderRequiresName verifies Kubernetes targets need a name and invalid
// options write nothing.
func TestRenderRequiresName(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := (format.Renderer{}).Render(
		&buffer, "", sampleEntries(), env.EmitOptions{Target: env.TargetKubernetes},
	)
	if !errors.Is(err, format.ErrMissingNameBase) || buffer.Len() != 0 {
		t.Errorf("Render = %v, wrote %q", err, buffer.String())
	}
	err = (format.Renderer{}).Render(
		&buffer, "api", sampleEntries(), env.EmitOptions{Target: "toml"},
	)
	if !errors.Is(err, format.ErrUnknownTarget) || buffer.Len() != 0 {
		t.Errorf("Render = %v, wrote %q", err, buffer.String())
	}
}
