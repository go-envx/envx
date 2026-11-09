package secrets_test

import (
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// TestAudit verifies the zero scope reports stored encodings, keypairs, and
// groups missing a public key.
func TestAudit(t *testing.T) {
	t.Parallel()
	f := newFixture(t, secrets.Group{
		Name:   "orphan",
		Values: map[string]string{"PLAIN": "value", "OTHER": "encrypted-naclbox:AA"},
	})
	f.generate(t, "api")
	f.set(t, "api", "TOKEN", "v")

	audit, err := f.service.Audit(secrets.AuditScope{})
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	wantSecrets := []secrets.StoredSecret{
		{Reference: secrets.Reference{Group: "api", Key: "TOKEN"}, Encrypted: true},
		{
			Reference:         secrets.Reference{Group: "orphan", Key: "OTHER"},
			Encrypted:         true,
			AlgorithmMismatch: true,
		},
		{Reference: secrets.Reference{Group: "orphan", Key: "PLAIN"}},
	}
	if !slices.Equal(audit.Secrets, wantSecrets) {
		t.Errorf("Secrets = %+v, want %+v", audit.Secrets, wantSecrets)
	}
	if len(audit.Keypairs) != 1 || audit.Keypairs[0].Group != "api" {
		t.Errorf("Keypairs = %+v", audit.Keypairs)
	}
	if !slices.Equal(audit.GroupsMissingPublicKey, []string{"orphan"}) {
		t.Errorf("GroupsMissingPublicKey = %v", audit.GroupsMissingPublicKey)
	}
}

// TestAuditScopeSkipsPrivateKeys verifies sections that do not need private
// keys never resolve them.
func TestAuditScopeSkipsPrivateKeys(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")
	f.set(t, "api", "TOKEN", "v")
	f.keys.resolves = 0

	scope := secrets.AuditScope{Secrets: true, MissingPublicKeys: true}
	audit, err := f.service.Audit(scope)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if f.keys.resolves != 0 {
		t.Errorf("resolved private keys %d times, want 0", f.keys.resolves)
	}
	if len(audit.Keypairs) != 0 || len(audit.Secrets) != 1 {
		t.Errorf("audit = %+v", audit)
	}
}
