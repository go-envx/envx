package validate

import (
	"slices"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
)

const (
	// GroupStore checks read only the store audit: offline and instant.
	GroupStore Group = "store"
	// GroupProvider checks the external providers the merge references, offline.
	GroupProvider Group = "provider"
	// GroupResolution checks read each project's merge in each environment.
	GroupResolution Group = "resolution"
)

// groupOrder ranks groups for sorting findings.
var groupOrder = []Group{GroupStore, GroupProvider, GroupResolution}

// storeCodes are reported by the store audit; every other diagnostic code is
// reported by the merge.
var storeCodes = []diagnostic.Code{
	diagnostic.SecretIsNotEncrypted,
	diagnostic.SecretAlgorithmMismatch,
	diagnostic.SecretIsNotReferenced,
	diagnostic.PublicKeyIsMissing,
	diagnostic.PrivateKeyIsInvalid,
	diagnostic.PrivateKeyIsUnavailable,
}

// Group names the source that owns a check's findings, which is also its cost
// boundary.
type Group string

// Check is one entry of the authoritative validation catalog.
type Check struct {
	// Code is the diagnostic code the check reports and is graded by.
	Code diagnostic.Code
	// Group is the source that owns the check's findings.
	Group Group
	// Summary is a short description for help text.
	Summary string
	// RequiresMerge reports whether the check reads the per-environment merge.
	RequiresMerge bool
}

// Checks returns every diagnostic code as a check, in definition order.
func Checks() []Check {
	defs := diagnostic.Definitions()
	checks := make([]Check, len(defs))
	for i, def := range defs {
		checks[i] = Check{
			Code:          def.Code,
			Group:         owner(def.Code),
			Summary:       def.Summary,
			RequiresMerge: requiresMerge(def.Code),
		}
	}
	return checks
}

// owner returns the group that reports code; empty for an unknown code.
func owner(code diagnostic.Code) Group {
	if _, ok := diagnostic.Lookup(code); !ok {
		return ""
	}
	if slices.Contains(storeCodes, code) {
		return GroupStore
	}
	if code == diagnostic.ExternalProviderUnavailable {
		return GroupProvider
	}
	return GroupResolution
}

// requiresMerge reports whether code's check reads the merge; unreferenced
// values and referenced providers need the references the merge collects.
func requiresMerge(code diagnostic.Code) bool {
	return owner(code) != GroupStore || code == diagnostic.SecretIsNotReferenced
}
