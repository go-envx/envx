package privatekey

import (
	"errors"
	"strings"
	"testing"
)

type fakeRepository struct {
	keys   map[string]string
	err    error
	origin string
}

func (f *fakeRepository) Origin() string {
	if f.origin != "" {
		return f.origin
	}
	return "fake-store"
}

func (f *fakeRepository) GetPrivateKey(group string) (
	key string, found bool, err error,
) {
	if f.err != nil {
		return "", false, f.err
	}
	if f.keys == nil {
		return "", false, nil
	}
	val, ok := f.keys[strings.ToUpper(group)]
	return val, ok, nil
}

func (f *fakeRepository) SetPrivateKey(group, privateKey string) error {
	if f.err != nil {
		return f.err
	}
	if f.keys == nil {
		f.keys = make(map[string]string)
	}
	f.keys[strings.ToUpper(group)] = privateKey
	return nil
}

func lookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

// TestServicePrecedence verifies specific environment, combined environment,
// and repository inputs are consulted in order.
func TestServicePrecedence(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		keys: map[string]string{"PRODUCTION": "repo-value"},
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
			name:   "repository",
			env:    map[string]string{},
			group:  "PRODUCTION",
			want:   "repo-value",
			origin: "fake-store",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, err := NewService(ServiceParams{
				Repository: repo,
				LookupEnv:  lookup(tt.env),
			})
			if err != nil {
				t.Fatalf("NewService(): %v", err)
			}
			got, err := svc.Resolve(tt.group)
			if err != nil {
				t.Fatalf("Resolve(): %v", err)
			}
			if got.Value != tt.want || got.Origin != tt.origin {
				t.Errorf("Resolve() = %#v, want value %q from %q", got, tt.want, tt.origin)
			}
		})
	}
}

// TestServiceFailsClosed verifies malformed, empty, and duplicate entries stop
// resolution with ErrInvalidKey instead of falling through to a lower-priority
// input.
func TestServiceFailsClosed(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		keys: map[string]string{"PRODUCTION": "repo-value"},
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
		{
			name: "combined empty value",
			env:  map[string]string{"ENVX_PRIVATE_KEY": "PRODUCTION=  "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, err := NewService(ServiceParams{
				Repository: repo,
				LookupEnv:  lookup(tt.env),
			})
			if err != nil {
				t.Fatalf("NewService(): %v", err)
			}
			_, err = svc.Resolve("production")
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Resolve() error = %v, want ErrInvalidKey", err)
			}
		})
	}
}

// TestServiceNotAvailableIsDistinct verifies a missing key can be distinguished
// from malformed service input.
func TestServiceNotAvailableIsDistinct(t *testing.T) {
	t.Parallel()

	svc, err := NewService(ServiceParams{
		LookupEnv: lookup(map[string]string{}),
	})
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	_, err = svc.Resolve("production")
	if !errors.Is(err, ErrNotAvailable) {
		t.Fatalf("Resolve() error = %v, want ErrNotAvailable", err)
	}

	repo := &fakeRepository{keys: map[string]string{}}
	svcWithRepo, err := NewService(ServiceParams{
		Repository: repo,
		LookupEnv:  lookup(map[string]string{}),
	})
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	_, err = svcWithRepo.Resolve("production")
	if !errors.Is(err, ErrNotAvailable) {
		t.Fatalf("Resolve() error = %v, want ErrNotAvailable", err)
	}
}

// TestServiceRequiresLookupEnv verifies NewService rejects a nil LookupEnv.
func TestServiceRequiresLookupEnv(t *testing.T) {
	t.Parallel()

	_, err := NewService(ServiceParams{
		Repository: &fakeRepository{},
	})
	if err == nil || err.Error() != "lookupEnv is required" {
		t.Fatalf("NewService() error = %v, want lookupEnv is required", err)
	}
}

// TestServiceResolveRejectsInvalidGroup verifies group validation.
func TestServiceResolveRejectsInvalidGroup(t *testing.T) {
	t.Parallel()

	svc, err := NewService(ServiceParams{
		LookupEnv: lookup(map[string]string{}),
	})
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := svc.Resolve(""); err == nil {
		t.Fatal("Resolve() accepted empty group")
	}
	if _, err := svc.Resolve("A=B"); err == nil {
		t.Fatal("Resolve() accepted group with '='")
	}
	if _, err := svc.Resolve("A B"); err == nil {
		t.Fatal("Resolve() accepted group with space")
	}
}

// TestServiceSet verifies key validation and repository persistence.
func TestServiceSet(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	svc, err := NewService(ServiceParams{
		Repository: repo,
		LookupEnv:  lookup(map[string]string{}),
	})
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}

	if err := svc.Set("production", "secret-key-1"); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	got, found, err := repo.GetPrivateKey("production")
	if err != nil || !found || got != "secret-key-1" {
		t.Errorf("repo.GetPrivateKey() = (%q, %v), want (secret-key-1, true)", got, found)
	}

	// Rejects invalid entries
	if err := svc.Set("", "key"); err == nil {
		t.Error("Set() accepted empty group")
	}
	if err := svc.Set("production", ""); err == nil {
		t.Error("Set() accepted empty private key")
	}
	if err := svc.Set("production", "line\nbreak"); err == nil {
		t.Error("Set() accepted private key with newline")
	}

	// Service without repository fails
	svcNoRepo, err := NewService(ServiceParams{
		LookupEnv: lookup(map[string]string{}),
	})
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if err := svcNoRepo.Set("production", "key"); err == nil {
		t.Error("Set() without repository succeeded")
	}
}
