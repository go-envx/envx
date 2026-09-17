package validate

import (
	"strings"

	"github.com/go-envx/envx/app/internal/status"
)

// Group classifies a check by the files it reads, which is also its cost
// boundary. The store group reads only the secrets store (and the keys file) and
// is offline and instant; the resolution group merges each project against each
// environment and can later reach the network for providers. Selecting only
// store checks lets a pre-commit hook skip the resolution merge entirely.
type Group string

const (
	// GroupStore reads only the secrets store and keys file: offline and instant.
	GroupStore Group = "store"
	// GroupResolution merges each project against each declared environment.
	GroupResolution Group = "resolution"
)

// Check is one validation check's registry entry. It is the single source of
// truth for a check's canonical status code, the group it belongs to, whether it
// reads the per-environment merge, and the human summary shown in flag help.
type Check struct {
	// Code is the canonical status code the check emits and is graded by.
	Code string
	// Group is the file/cost boundary the check belongs to.
	Group Group
	// RequiresMerge reports whether the check reads the per-environment merge.
	// Every resolution check needs it, and so does the unused-secret store check,
	// which needs the reference set the merge collects; the remaining store checks
	// read only the store and leave it false.
	RequiresMerge bool
	// Summary is a short human description used in the selection flag's help text.
	Summary string
}

// registry lists every check exactly once, in the order findings are grouped for
// display: the offline store checks first, then the resolution checks. It is the
// one place a check's group, cost, and identity are declared. Both the selection
// flag names and the envx.yaml severity keys derive from each Code, so the flag
// and the config property can never drift apart.
var registry = []Check{
	{
		Code:    status.SecretIsNotEncrypted,
		Group:   GroupStore,
		Summary: "a stored value left unencrypted",
	},
	{
		Code:    status.SecretAlgorithmMismatch,
		Group:   GroupStore,
		Summary: "a value stored under a non-configured algorithm",
	},
	{
		Code:          status.SecretIsNotReferenced,
		Group:         GroupStore,
		RequiresMerge: true,
		Summary:       "a stored value no environment references",
	},
	{
		Code:    status.PublicKeyIsMissing,
		Group:   GroupStore,
		Summary: "a group with stored secrets but no public key",
	},
	{
		Code:    status.PrivateKeyIsInvalid,
		Group:   GroupStore,
		Summary: "a group whose private key is malformed or mismatched",
	},
	{
		Code:    status.PrivateKeyIsUnavailable,
		Group:   GroupStore,
		Summary: "a group with no private key available in this context",
	},
	{
		Code:          status.SecretReferenceNotFound,
		Group:         GroupResolution,
		RequiresMerge: true,
		Summary:       "a reference to a value absent from the store",
	},
	{
		Code:          status.InvalidSecretReference,
		Group:         GroupResolution,
		RequiresMerge: true,
		Summary:       "a malformed secret reference",
	},
	{
		Code:          status.SecretReferenceIsUnresolved,
		Group:         GroupResolution,
		RequiresMerge: true,
		Summary:       "a reference that passed every store check yet failed to decrypt",
	},
	{
		Code:          status.CircularVariableReference,
		Group:         GroupResolution,
		RequiresMerge: true,
		Summary:       "a variable substitution cycle",
	},
	{
		Code:          status.UnresolvedVariableReference,
		Group:         GroupResolution,
		RequiresMerge: true,
		Summary:       "a substitution of an undefined variable",
	},
	{
		Code:          status.PropertyNotDeclaredInBase,
		Group:         GroupResolution,
		RequiresMerge: true,
		Summary:       "an overlay key its namespace base file never declares",
	},
}

// byCode indexes the registry by canonical code for O(1) group lookup.
var byCode = func() map[string]Check {
	index := make(map[string]Check, len(registry))
	for _, check := range registry {
		index[check.Code] = check
	}
	return index
}()

// Checks returns a copy of the check registry in display order. Callers
// registering the selection flags iterate it so the flag surface always matches
// the checks the engine runs.
func Checks() []Check {
	out := make([]Check, len(registry))
	copy(out, registry)
	return out
}

// FlagName returns a check's selection flag name: the lowercased code with
// underscores turned into dashes, e.g. SECRET_IS_NOT_ENCRYPTED ->
// "secret-is-not-encrypted". It is the kebab-case twin of the code's config
// severity key, so `validate --secret-is-not-encrypted` selects exactly the check
// whose severity is set by `validate.secret_is_not_encrypted` in envx.yaml.
func FlagName(code string) string {
	return strings.ReplaceAll(strings.ToLower(code), "_", "-")
}

// groupOf returns a code's group and whether it names a registered check. An
// unregistered code is not a check, so the resolution pass never records it.
func groupOf(code string) (Group, bool) {
	check, ok := byCode[code]
	return check.Group, ok
}

// groupOrder ranks the check groups for deterministic finding output: the
// offline store findings sort ahead of the resolution findings, matching the
// registry's declaration order.
var groupOrder = map[Group]int{
	GroupStore:      0,
	GroupResolution: 1,
}

// groupRank returns a code's sort rank by its check group, so findings cluster
// store-first then resolution. An unregistered code sorts after every known
// group, keeping the order total and deterministic.
func groupRank(code string) int {
	group, ok := groupOf(code)
	if !ok {
		return len(groupOrder)
	}
	if rank, ok := groupOrder[group]; ok {
		return rank
	}
	return len(groupOrder)
}

// runs reports whether the check with the given code should run. With no
// selection every check runs; with a selection only the named checks run, so a
// hook pays only for the checks it lists.
func (p Params) runs(code string) bool {
	if len(p.Selected) == 0 {
		return true
	}
	return p.Selected[code]
}

// needsMerge reports whether any selected check reads the per-environment merge.
// A store-only selection returns false, so validate performs no merge and no
// network I/O for a pre-commit hook that runs only the offline store checks.
func (p Params) needsMerge() bool {
	for _, check := range registry {
		if check.RequiresMerge && p.runs(check.Code) {
			return true
		}
	}
	return false
}
