package flags

import "testing"

// -------------------------------------------------------------------------------------
// TestSpecHelpText verifies that HelpText appends the ENVX_* hint only when the
// flag declares an env-var fallback.
func TestSpecHelpText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{
			name: "with env var",
			spec: Spec{Name: "env", Env: "ENVX_ENV", Usage: "target environment"},
			want: "target environment (env: ENVX_ENV)",
		},
		{
			name: "without env var",
			spec: Spec{Name: "reveal", Usage: "print values instead of masking"},
			want: "print values instead of masking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.spec.HelpText(); got != tt.want {
				t.Errorf("HelpText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestCatalogEnvVarsUnique guards against two flags accidentally sharing an
// ENVX_* fallback, which would make env-driven config ambiguous.
func TestCatalogEnvVarsUnique(t *testing.T) {
	t.Parallel()

	specs := []Spec{
		Config, Env, Strict, Prefix, Suffix, NamespacePrefix, Overload,
		Reveal, Output,
	}
	seen := make(map[string]string)
	for _, s := range specs {
		if s.Env == "" {
			continue
		}
		if prev, ok := seen[s.Env]; ok {
			t.Errorf("env var %q used by both %q and %q", s.Env, prev, s.Name)
		}
		seen[s.Env] = s.Name
	}
}
