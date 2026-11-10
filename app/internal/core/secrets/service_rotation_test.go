package secrets_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// TestRotateReencryptsAndRetainsOldKey verifies rotation replaces the public
// key, re-encrypts every value, and keeps the old private key.
func TestRotateReencryptsAndRetainsOldKey(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	oldPublic := f.generate(t, "api")
	f.set(t, "api", "A", "1")
	f.set(t, "api", "B", "2")
	oldPrivate := f.keys.byPublic["api"][oldPublic]

	res, err := f.service.RotateKeypair("api")
	if err != nil {
		t.Fatalf("RotateKeypair: %v", err)
	}
	newPublic := res.Keypair.PublicKey
	if newPublic == oldPublic || f.store.groups["api"].PublicKey != newPublic {
		t.Errorf("public key not replaced")
	}
	want := []secrets.Reference{{Group: "api", Key: "A"}, {Group: "api", Key: "B"}}
	if !slices.Equal(res.Secrets, want) {
		t.Errorf("Secrets = %v, want %v", res.Secrets, want)
	}
	if f.keys.byPublic["api"][oldPublic] != oldPrivate {
		t.Error("old private key was not retained")
	}
	for key, plaintext := range map[string]string{"A": "1", "B": "2"} {
		got, err := f.service.GetSecret("api", key)
		if err != nil || got.Value != plaintext {
			t.Errorf("Get %s after rotation = %q, %v", key, got.Value, err)
		}
	}
}

// TestRotateFailuresLeaveStoreUnchanged verifies each rotation failure leaves
// the old public key and ciphertext in place.
func TestRotateFailuresLeaveStoreUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*fixture)
		wantErr string
		target  error
	}{
		{
			name:    "missing private key",
			setup:   func(f *fixture) { delete(f.keys.byPublic, "api") },
			wantErr: "not available",
		},
		{
			name: "plaintext value",
			setup: func(f *fixture) {
				group := f.store.groups["api"]
				group.Values["PLAIN"] = "value"
				f.store.put(group)
			},
			wantErr: "not encrypted",
		},
		{
			name: "shadowing override",
			setup: func(f *fixture) {
				group := f.store.groups["api"]
				f.keys.overrides["api"] = secrets.PrivateKey{
					Value:  f.keys.byPublic["api"][group.PublicKey],
					Origin: "ENVX_PRIVATE_KEY_API",
				}
			},
			wantErr: "ENVX_PRIVATE_KEY_API",
		},
		{
			name: "retention failure",
			setup: func(f *fixture) {
				f.keys.retainErr = errors.New("disk full")
			},
			wantErr: "disk full",
		},
		{
			name: "concurrent change",
			setup: func(f *fixture) {
				f.store.beforeCommit = func() {
					f.store.revisions["api"]++
				}
			},
			target: secrets.ErrConflict,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			oldPublic := f.generate(t, "api")
			f.set(t, "api", "A", "1")
			tt.setup(f)
			before := f.store.groups["api"]

			_, err := f.service.RotateKeypair("api")
			if err == nil {
				t.Fatal("RotateKeypair succeeded, want error")
			}
			if tt.target != nil && !errors.Is(err, tt.target) {
				t.Errorf("error = %v, want %v", err, tt.target)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want %q", err, tt.wantErr)
			}
			after := f.store.groups["api"]
			if after.PublicKey != oldPublic || after.Values["A"] != before.Values["A"] {
				t.Error("store changed after failed rotation")
			}
			if tt.name == "shadowing override" && len(f.keys.byPublic["api"]) != 1 {
				t.Error("retained a new key despite a shadowing override")
			}
		})
	}
}
