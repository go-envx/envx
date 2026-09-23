package validate

import (
	"testing"

	"github.com/go-envx/envx/app/internal/shared/status"
)

// TestCollectReferences verifies only well-formed secret references are recorded,
// with the group lowercased to match the store index, and plain or escaped values
// ignored.
func TestCollectReferences(t *testing.T) {
	t.Parallel()

	referenced := make(map[storeRef]bool)
	collectReferences(referenced, []string{
		"secret://Production/api_key",
		"plain-value",
		`\secret://production/escaped`,
		"secret://shared/token",
	})

	if !referenced[storeRef{group: "production", key: "api_key"}] {
		t.Error("production/api_key not recorded")
	}
	if !referenced[storeRef{group: "shared", key: "token"}] {
		t.Error("shared/token not recorded")
	}
	if referenced[storeRef{group: "production", key: "escaped"}] {
		t.Error("escaped literal must not be recorded as a reference")
	}
	if len(referenced) != 2 {
		t.Errorf("recorded %d references, want 2", len(referenced))
	}
}

// TestSortOrdersErrorsFirst verifies Sort places errors before warnings and then
// orders deterministically by check group and code, project, environment, and
// key.
func TestSortOrdersErrorsFirst(t *testing.T) {
	t.Parallel()

	report := Report{Findings: []Finding{
		{Code: status.SecretIsNotReferenced, Severity: SeverityWarning, Key: "shared/unused"},
		{
			Code: status.SecretReferenceNotFound, Severity: SeverityError,
			Project: "web", Environment: "prod", Key: "TOKEN",
		},
		{
			Code: status.SecretReferenceNotFound, Severity: SeverityError,
			Project: "api", Environment: "prod", Key: "PASSWORD",
		},
		{Code: status.SecretIsNotEncrypted, Severity: SeverityError, Key: "shared/leaked"},
	}}
	report.Sort()

	// Errors come first; within errors the store-group plaintext code sorts ahead
	// of the resolution-group reference code, so the warning orphan lands last.
	if report.Findings[0].Severity != SeverityError {
		t.Fatalf("first finding severity = %q, want error", report.Findings[0].Severity)
	}
	if first := report.Findings[0].Code; first != status.SecretIsNotEncrypted {
		t.Errorf("first error code = %q, want the store-group plaintext code", first)
	}
	last := report.Findings[len(report.Findings)-1]
	if last.Severity != SeverityWarning {
		t.Errorf("last finding severity = %q, want warning", last.Severity)
	}

	// The two reference errors sort by project: api before web.
	var refKeys []string
	for _, f := range report.Findings {
		if f.Code == status.SecretReferenceNotFound {
			refKeys = append(refKeys, f.Project)
		}
	}
	if len(refKeys) != 2 || refKeys[0] != "api" || refKeys[1] != "web" {
		t.Errorf("reference projects = %v, want [api web]", refKeys)
	}
}
