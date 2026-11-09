package secrets_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// TestParseReference verifies references are normalized, non-references are
// distinguished, and malformed references are rejected.
func TestParseReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw     string
		want    secrets.Reference
		notRef  bool
		wantErr bool
	}{
		{raw: "secret://API/Token", want: secrets.Reference{Group: "api", Key: "Token"}},
		{raw: "plain", notRef: true},
		{raw: `\secret://api/key`, notRef: true},
		{raw: "postgres://host/db", notRef: true},
		{raw: "secret://", wantErr: true},
		{raw: "secret://api", wantErr: true},
		{raw: "secret://api/", wantErr: true},
		{raw: "secret:///key", wantErr: true},
		{raw: "secret://api/a/b", wantErr: true},
		{raw: "secret://a b/K", wantErr: true},
		{raw: "secret://#api/K", wantErr: true},
	}
	for _, tt := range tests {
		got, err := secrets.ParseReference(tt.raw, "")
		switch {
		case tt.notRef:
			if !errors.Is(err, secrets.ErrNotReference) {
				t.Errorf("ParseReference(%q) error = %v, want ErrNotReference", tt.raw, err)
			}
		case tt.wantErr:
			if err == nil || errors.Is(err, secrets.ErrNotReference) {
				t.Errorf("ParseReference(%q) error = %v, want malformed", tt.raw, err)
			}
		case err != nil || got != tt.want:
			t.Errorf("ParseReference(%q) = %+v, %v, want %+v", tt.raw, got, err, tt.want)
		}
	}
}

// FuzzParseReference verifies every parsed reference round-trips through its
// canonical string form.
func FuzzParseReference(f *testing.F) {
	f.Add("secret://api/TOKEN")
	f.Add("secret://a/b/c")
	f.Add("plain")
	f.Fuzz(func(t *testing.T, raw string) {
		ref, err := secrets.ParseReference(raw, "")
		if err != nil {
			return
		}
		again, err := secrets.ParseReference(ref.String(), "")
		if err != nil || again != ref {
			t.Errorf("round trip of %q = %+v, %v; want %+v", raw, again, err, ref)
		}
	})
}

// TestNormalizeGroup verifies group names are lowercased and names that would
// break references, key files, or environment variables are rejected.
func TestNormalizeGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		group   string
		want    string
		wantErr bool
	}{
		{group: "API", want: "api"},
		{group: "my-app_1", want: "my-app_1"},
		{group: "", wantErr: true},
		{group: "  ", wantErr: true},
		{group: "a b", wantErr: true},
		{group: "a/b", wantErr: true},
		{group: "a=b", wantErr: true},
		{group: "#api", wantErr: true},
	}
	for _, tt := range tests {
		got, err := secrets.NormalizeGroup(tt.group)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("NormalizeGroup(%q) = %q, %v", tt.group, got, err)
		}
	}
}

// TestValidateKey verifies empty keys and keys that would break references
// are rejected.
func TestValidateKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key     string
		wantErr bool
	}{
		{key: "DB_PASSWORD"},
		{key: "with space"},
		{key: "", wantErr: true},
		{key: "a/b", wantErr: true},
		{key: "a\nb", wantErr: true},
	}
	for _, tt := range tests {
		if err := secrets.ValidateKey(tt.key); (err != nil) != tt.wantErr {
			t.Errorf("ValidateKey(%q) = %v, wantErr %v", tt.key, err, tt.wantErr)
		}
	}
}
