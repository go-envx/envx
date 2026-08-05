package privatekey

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------

// lookup builds a deterministic environment lookup for source tests.
func lookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

// -------------------------------------------------------------------------------------

// TestResolverPrecedence verifies specific environment, combined environment,
// and file inputs are consulted in order.
func TestResolverPrecedence(t *testing.T) {
	t.Parallel()

	keysPath := filepath.Join(t.TempDir(), "envx.keys")
	if err := os.WriteFile(
		keysPath, []byte("PRODUCTION=file-value\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		env    map[string]string
		group  string
		want   string
		origin string
	}{
		{
			name:   "specific environment",
			env:    map[string]string{"ENVX_PRIVATE_KEY_PRODUCTION": "specific-value"},
			group:  "production",
			want:   "specific-value",
			origin: "ENVX_PRIVATE_KEY_PRODUCTION",
		},
		{
			name: "combined environment",
			env: map[string]string{
				"ENVX_PRIVATE_KEY": "SHARED=combined-value\nPRODUCTION=combined-production",
			},
			group:  "production",
			want:   "combined-production",
			origin: "ENVX_PRIVATE_KEY",
		},
		{
			name:   "file",
			env:    map[string]string{},
			group:  "PRODUCTION",
			want:   "file-value",
			origin: keysPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolver := NewResolver(ResolverOptions{
				KeysPath:  keysPath,
				LookupEnv: lookup(tt.env),
			})
			got, err := resolver.Resolve(tt.group)
			if err != nil {
				t.Fatalf("Resolve(): %v", err)
			}
			if got.Value != tt.want || got.Origin != tt.origin {
				t.Errorf("Resolve() = %#v, want value %q from %q", got, tt.want, tt.origin)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestResolverFailsClosed verifies malformed, empty, and duplicate entries stop
// resolution instead of falling through to a lower-priority input.
func TestResolverFailsClosed(t *testing.T) {
	t.Parallel()

	keysPath := filepath.Join(t.TempDir(), "envx.keys")
	if err := os.WriteFile(
		keysPath, []byte("PRODUCTION=file-value\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "specific empty",
			env:  map[string]string{"ENVX_PRIVATE_KEY_PRODUCTION": ""},
		},
		{
			name: "combined malformed",
			env:  map[string]string{"ENVX_PRIVATE_KEY": "not-an-entry"},
		},
		{
			name: "combined duplicate",
			env:  map[string]string{"ENVX_PRIVATE_KEY": "PRODUCTION=one\nproduction=two"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolver := NewResolver(ResolverOptions{
				KeysPath:  keysPath,
				LookupEnv: lookup(tt.env),
			})
			if _, err := resolver.Resolve("production"); err == nil {
				t.Fatal("Resolve() accepted a malformed higher-priority source")
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestResolverNotAvailableIsDistinct verifies a missing key can be distinguished
// from malformed resolver input.
func TestResolverNotAvailableIsDistinct(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(ResolverOptions{LookupEnv: lookup(map[string]string{})})
	_, err := resolver.Resolve("production")
	if !errors.Is(err, ErrNotAvailable) {
		t.Fatalf("Resolve() error = %v, want ErrNotAvailable", err)
	}
}
