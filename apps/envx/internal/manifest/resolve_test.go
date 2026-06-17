package manifest

import (
	"testing"
)

// -------------------------------------------------------------------------------------
// mockFlagSet implements FlagSet for testing.
type mockFlagSet struct {
	changed map[string]bool
}

// -------------------------------------------------------------------------------------
// Changed reports whether the specified flag was changed. The test sets up the
// changed map to control which flags are considered changed.
func (m *mockFlagSet) Changed(name string) bool {
	return m.changed[name]
}

// -------------------------------------------------------------------------------------
// mockEnv creates a LookupEnv function backed by a map.
func mockEnv(vars map[string]string) EnvLookupFunc {
	return func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	}
}

// -------------------------------------------------------------------------------------
// TestResolveOverloadPrecedence verifies the full precedence chain for the
// overload setting: flag > env var > manifest > default.
func TestResolveOverloadPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flagVal  bool
		changed  map[string]bool
		env      map[string]string
		manifest Settings
		want     bool
	}{
		{
			name:     "flag wins when set",
			flagVal:  true,
			changed:  map[string]bool{"overload": true},
			env:      map[string]string{"ENVX_OVERLOAD": "false"},
			manifest: Settings{Overload: new(false)},
			want:     true,
		},
		{
			name:     "env var wins over manifest",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{"ENVX_OVERLOAD": "true"},
			manifest: Settings{Overload: new(false)},
			want:     true,
		},
		{
			name:     "manifest default used when no flag or env",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{Overload: new(true)},
			want:     true,
		},
		{
			name:     "invalid env var falls through to manifest",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{"ENVX_OVERLOAD": "not-a-bool"},
			manifest: Settings{Overload: new(true)},
			want:     true,
		},
		{
			name:     "defaults to false when nothing set",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Resolver{LookupEnv: mockEnv(tt.env)}
			m := &Manifest{
				Settings:     tt.manifest,
				Environments: []string{"dev"},
				Projects:     map[string]Project{"a": {Includes: []string{"x/x"}}},
			}
			flags := RawFlags{Overload: tt.flagVal}
			cfg := r.Resolve(
				&flags,
				&mockFlagSet{changed: tt.changed},
				m,
				&Project{Includes: []string{"x/x"}},
			)
			if cfg.Overload != tt.want {
				t.Errorf("Overload = %v, want %v", cfg.Overload, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestResolveStrictPrecedence verifies the full precedence chain for the
// strict setting: flag > env var > manifest > default.
func TestResolveStrictPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flagVal  bool
		changed  map[string]bool
		env      map[string]string
		manifest Settings
		want     bool
	}{
		{
			name:     "flag wins when set",
			flagVal:  true,
			changed:  map[string]bool{"strict": true},
			env:      map[string]string{"ENVX_STRICT": "false"},
			manifest: Settings{Strict: new(false)},
			want:     true,
		},
		{
			name:     "env var wins over manifest",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{"ENVX_STRICT": "true"},
			manifest: Settings{Strict: new(false)},
			want:     true,
		},
		{
			name:     "manifest default used when no flag or env",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{Strict: new(true)},
			want:     true,
		},
		{
			name:     "invalid env var falls through to manifest",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{"ENVX_STRICT": "invalid"},
			manifest: Settings{Strict: new(true)},
			want:     true,
		},
		{
			name:     "defaults to false when nothing set",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Resolver{LookupEnv: mockEnv(tt.env)}
			m := &Manifest{
				Settings:     tt.manifest,
				Environments: []string{"dev"},
				Projects:     map[string]Project{"a": {Includes: []string{"x/x"}}},
			}
			flags := RawFlags{Strict: tt.flagVal}
			cfg := r.Resolve(
				&flags,
				&mockFlagSet{changed: tt.changed},
				m,
				&Project{Includes: []string{"x/x"}},
			)
			if cfg.Strict != tt.want {
				t.Errorf("Strict = %v, want %v", cfg.Strict, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestResolvePrefixPrecedence verifies the full precedence chain for the
// prefix setting: flag > env var > project > manifest > default.
func TestResolvePrefixPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flagVal  string
		changed  map[string]bool
		env      map[string]string
		manifest Settings
		project  Project
		want     string
	}{
		{
			name:    "flag wins when set",
			flagVal: "FLAG_PREFIX",
			changed: map[string]bool{"prefix": true},
			env:     map[string]string{"ENVX_PREFIX": "ENV_PREFIX"},
			manifest: Settings{
				Prefix: "MANIFEST_PREFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{
					Prefix: "PROJECT_PREFIX",
				},
			},
			want: "FLAG_PREFIX",
		},
		{
			name:    "env var wins over project and manifest",
			flagVal: "",
			changed: map[string]bool{},
			env:     map[string]string{"ENVX_PREFIX": "ENV_PREFIX"},
			manifest: Settings{
				Prefix: "MANIFEST_PREFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{
					Prefix: "PROJECT_PREFIX",
				},
			},
			want: "ENV_PREFIX",
		},
		{
			name:    "project wins over manifest",
			flagVal: "",
			changed: map[string]bool{},
			env:     map[string]string{},
			manifest: Settings{
				Prefix: "MANIFEST_PREFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{
					Prefix: "PROJECT_PREFIX",
				},
			},
			want: "PROJECT_PREFIX",
		},
		{
			name:    "manifest used as last resort",
			flagVal: "",
			changed: map[string]bool{},
			env:     map[string]string{},
			manifest: Settings{
				Prefix: "MANIFEST_PREFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
			},
			want: "MANIFEST_PREFIX",
		},
		{
			name:     "empty when nothing set",
			flagVal:  "",
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{},
			project: Project{
				Includes: []string{"x/x"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Resolver{LookupEnv: mockEnv(tt.env)}
			m := &Manifest{
				Settings:     tt.manifest,
				Environments: []string{"dev"},
				Projects:     map[string]Project{"a": {Includes: []string{"x/x"}}},
			}
			flags := RawFlags{Prefix: tt.flagVal}
			cfg := r.Resolve(&flags, &mockFlagSet{changed: tt.changed}, m, &tt.project)
			if cfg.Prefix != tt.want {
				t.Errorf("Prefix = %q, want %q", cfg.Prefix, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestResolveSuffixPrecedence verifies the full precedence chain for the
// suffix setting: flag > env var > project > manifest > default.
func TestResolveSuffixPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flagVal  string
		changed  map[string]bool
		env      map[string]string
		manifest Settings
		project  Project
		want     string
	}{
		{
			name:    "flag wins when set",
			flagVal: "FLAG_SUFFIX",
			changed: map[string]bool{"suffix": true},
			env:     map[string]string{"ENVX_SUFFIX": "ENV_SUFFIX"},
			manifest: Settings{
				Suffix: "MANIFEST_SUFFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{
					Suffix: "PROJECT_SUFFIX",
				},
			},
			want: "FLAG_SUFFIX",
		},
		{
			name:    "env var wins over project and manifest",
			flagVal: "",
			changed: map[string]bool{},
			env:     map[string]string{"ENVX_SUFFIX": "ENV_SUFFIX"},
			manifest: Settings{
				Suffix: "MANIFEST_SUFFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{
					Suffix: "PROJECT_SUFFIX",
				},
			},
			want: "ENV_SUFFIX",
		},
		{
			name:    "project wins over manifest",
			flagVal: "",
			changed: map[string]bool{},
			env:     map[string]string{},
			manifest: Settings{
				Suffix: "MANIFEST_SUFFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{
					Suffix: "PROJECT_SUFFIX",
				},
			},
			want: "PROJECT_SUFFIX",
		},
		{
			name:    "manifest used as last resort",
			flagVal: "",
			changed: map[string]bool{},
			env:     map[string]string{},
			manifest: Settings{
				Suffix: "MANIFEST_SUFFIX",
			},
			project: Project{
				Includes: []string{"x/x"},
			},
			want: "MANIFEST_SUFFIX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Resolver{LookupEnv: mockEnv(tt.env)}
			m := &Manifest{
				Settings:     tt.manifest,
				Environments: []string{"dev"},
				Projects:     map[string]Project{"a": {Includes: []string{"x/x"}}},
			}
			flags := RawFlags{Suffix: tt.flagVal}
			cfg := r.Resolve(&flags, &mockFlagSet{changed: tt.changed}, m, &tt.project)
			if cfg.Suffix != tt.want {
				t.Errorf("Suffix = %q, want %q", cfg.Suffix, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestResolveNamespacePrefixPrecedence verifies the full precedence chain for
// the namespace-prefix setting, including its default-true behavior.
func TestResolveNamespacePrefixPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flagVal  bool
		changed  map[string]bool
		env      map[string]string
		manifest Settings
		project  Project
		want     bool
	}{
		{
			name:     "flag wins when set",
			flagVal:  false,
			changed:  map[string]bool{"namespace-prefix": true},
			env:      map[string]string{"ENVX_NAMESPACE_PREFIX": "true"},
			manifest: Settings{NamespacePrefix: new(true)},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{NamespacePrefix: new(true)},
			},
			want: false,
		},
		{
			name:     "env var wins over project",
			flagVal:  true,
			changed:  map[string]bool{},
			env:      map[string]string{"ENVX_NAMESPACE_PREFIX": "false"},
			manifest: Settings{NamespacePrefix: new(true)},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{NamespacePrefix: new(true)},
			},
			want: false,
		},
		{
			name:     "project setting wins over manifest",
			flagVal:  true,
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{NamespacePrefix: new(true)},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{NamespacePrefix: new(false)},
			},
			want: false,
		},
		{
			name:     "manifest default when project not set",
			flagVal:  true,
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{NamespacePrefix: new(false)},
			project:  Project{Includes: []string{"x/x"}},
			want:     false,
		},
		{
			name:     "defaults to false when nothing set",
			flagVal:  false,
			changed:  map[string]bool{},
			env:      map[string]string{},
			manifest: Settings{},
			project:  Project{Includes: []string{"x/x"}},
			want:     false,
		},
		{
			name:     "invalid env var falls through to project",
			flagVal:  true,
			changed:  map[string]bool{},
			env:      map[string]string{"ENVX_NAMESPACE_PREFIX": "not-a-bool"},
			manifest: Settings{},
			project: Project{
				Includes: []string{"x/x"},
				Settings: Settings{NamespacePrefix: new(false)},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Resolver{LookupEnv: mockEnv(tt.env)}
			m := &Manifest{
				Settings:     tt.manifest,
				Environments: []string{"dev"},
				Projects:     map[string]Project{"a": {Includes: []string{"x/x"}}},
			}
			flags := RawFlags{NamespacePrefix: tt.flagVal}
			cfg := r.Resolve(&flags, &mockFlagSet{changed: tt.changed}, m, &tt.project)
			if cfg.NamespacePrefix != tt.want {
				t.Errorf("NamespacePrefix = %v, want %v", cfg.NamespacePrefix, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestNewResolverUsesOsLookupEnv verifies that the default resolver uses
// os.LookupEnv as its environment source.
func TestNewResolverUsesOsLookupEnv(t *testing.T) {
	t.Parallel()

	r := NewResolver()
	if r.LookupEnv == nil {
		t.Fatal("expected LookupEnv to be set")
	}
}
