package validate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// ProjectManager pairs a project name with the envmerge Manager that resolves
// it, so validate can diagnose every project without depending on the config
// package.
type ProjectManager struct {
	// Name is the manifest project name, used to attribute findings.
	Name string
	// Manager resolves and diagnoses the project's environments.
	Manager *envmerge.Manager
}

// Workspace is the resolved input Validate iterates: one manager per project,
// the declared environments, and the shared secrets manager. A nil Secrets
// manager skips the store-level checks, running only per-environment resolution.
type Workspace struct {
	// Projects is every project to diagnose.
	Projects []ProjectManager
	// Environments is the declared environment list every project is diagnosed
	// against.
	Environments []string
	// Secrets provides the store-level findings; nil skips them.
	Secrets *secrets.Manager
}

// storeRef identifies one stored secret for orphan matching. The group is
// lowercased to match how references index the store; the key is verbatim.
type storeRef struct {
	// group is the lowercased key-group name.
	group string
	// key is the entry name within the group.
	key string
}

// Validate diagnoses every project against every declared environment, collects
// the references each configuration uses, and adds the store-level findings no
// single environment can see: plaintext values, orphaned values, and broken or
// unavailable keypairs. It never materializes plaintext — every reference is
// diagnosed through the masked dry-run path — and never aborts on a per-value
// failure: every finding is collected and the verdict is graded at the end.
// Structural failures (a malformed manifest or unreadable YAML) are returned as
// errors because there is no valid workspace to grade.
func Validate(w Workspace, params Params) (Report, error) {
	var report Report

	// Diagnose each project × environment, collecting reference findings and the
	// set of stored secrets any configuration actually references. The merge runs
	// only when a selected check needs it, so a store-only selection performs no
	// merge and no network I/O.
	referenced := make(map[storeRef]bool)
	if params.needsMerge() {
		for _, project := range w.Projects {
			for _, environment := range w.Environments {
				if err := diagnoseEnvironment(
					&report, params, referenced, project, environment,
				); err != nil {
					return Report{}, err
				}
			}
		}
	}

	// Add the store-level findings the per-environment view cannot produce.
	if w.Secrets != nil {
		if err := addStoreFindings(&report, params, w.Secrets, referenced); err != nil {
			return Report{}, err
		}
	}

	report.grade(params.Strict)
	return report, nil
}

// diagnoseEnvironment explains one project in one environment, appending a
// finding for every non-OK resolution and recording every secret reference the
// environment uses so orphan detection can subtract it from the store.
func diagnoseEnvironment(
	report *Report,
	params Params,
	referenced map[storeRef]bool,
	project ProjectManager,
	environment string,
) error {
	explanation, err := project.Manager.Explain(envmerge.ExplainParams{
		Environment: environment,
		Reveal:      false,
	})
	if err != nil {
		return fmt.Errorf(
			"diagnosing project %q environment %q: %w", project.Name, environment, err,
		)
	}

	for i := range explanation.Entries {
		entry := &explanation.Entries[i]
		collectReferences(referenced, entry.Items)

		// The base-declaration check reads only provenance, so it runs on the same
		// merge regardless of how the value resolved.
		if params.runs(status.PropertyNotDeclaredInBase) &&
			!declaredInBase(entry.Origin, environment) {
			report.record(params, Finding{
				Project:     project.Name,
				Environment: environment,
				Key:         entry.Key,
				Code:        status.PropertyNotDeclaredInBase,
				Message:     "declared only in an environment overlay, not the base file",
			})
		}

		if entry.Resolution.Severity == envmerge.SeverityOK {
			continue
		}
		// The resolution group reports only resolution-structure codes; a store-owned
		// cause (plaintext, algorithm mismatch, key health) is reported once by the
		// store group and never re-reported here, so no finding is duplicated.
		group, ok := groupOf(entry.Resolution.Code)
		if !ok || group != GroupResolution || !params.runs(entry.Resolution.Code) {
			continue
		}
		report.record(params, Finding{
			Project:     project.Name,
			Environment: environment,
			Key:         entry.Key,
			Code:        entry.Resolution.Code,
			Message:     entry.Resolution.Message,
		})
	}
	return nil
}

// declaredInBase reports whether a key's provenance includes a namespace base
// file. A key present only in environment overlays is not declared in any base,
// so the base is no longer the single catalog of every key and an overlay-key
// typo would silently create a new key. Any base source among the winner or the
// shadowed sources satisfies the check.
func declaredInBase(origin envmerge.Origin, environment string) bool {
	if isBaseSource(origin.Winner, environment) {
		return true
	}
	for _, source := range origin.Shadowed {
		if isBaseSource(source, environment) {
			return true
		}
	}
	return false
}

// isBaseSource reports whether a provenance source is a namespace base file
// (<name>.yaml) rather than an environment overlay (<name>.<env>.yaml) or the OS
// environment. The overlay for this environment ends with ".<env>.yaml", so a
// real YAML file that does not is a base declaration.
func isBaseSource(source envmerge.Source, environment string) bool {
	if !strings.HasSuffix(source.File, ".yaml") {
		return false
	}
	return !strings.HasSuffix(source.File, "."+environment+".yaml")
}

// collectReferences records every secret reference among a leaf's raw items so a
// stored value referenced by any environment is not later reported as orphaned.
func collectReferences(referenced map[storeRef]bool, items []string) {
	for _, item := range items {
		if group, key, ok := secrets.ParseReference(item); ok {
			referenced[storeRef{group: group, key: key}] = true
		}
	}
}

// addStoreFindings appends the offline, store-only findings, reading the store
// without decrypting any value. It splits the work by the store artifact each
// check reads — the stored secrets and the group keypairs — and skips an artifact
// entirely when none of its checks are selected.
func addStoreFindings(
	report *Report, params Params, manager *secrets.Manager, referenced map[storeRef]bool,
) error {
	// The encryption, algorithm, orphan, and missing-public-key checks all read the
	// stored secrets; read them once when any of those checks runs.
	if params.runs(status.SecretIsNotEncrypted) ||
		params.runs(status.SecretAlgorithmMismatch) ||
		params.runs(status.SecretIsNotReferenced) ||
		params.runs(status.PublicKeyIsMissing) {
		if err := addSecretFindings(report, params, manager, referenced); err != nil {
			return err
		}
	}

	// The keypair-health checks read the group keypairs.
	if params.runs(status.PrivateKeyIsInvalid) ||
		params.runs(status.PrivateKeyIsUnavailable) {
		if err := addKeypairFindings(report, params, manager); err != nil {
			return err
		}
	}
	return nil
}

// addSecretFindings reports the store-value checks — plaintext, algorithm
// mismatch, orphaned, and missing public key — reading the store once without
// decrypting any value. Each finding is gated by its own selection so a hook pays
// only for the checks it lists, and the orphan check reuses the referenced set
// gathered during the per-environment merge.
func addSecretFindings(
	report *Report, params Params, manager *secrets.Manager, referenced map[storeRef]bool,
) error {
	stored, err := manager.StoredSecrets()
	if err != nil {
		return fmt.Errorf("reading secrets store: %w", err)
	}
	for _, secret := range stored {
		identity := secret.Group + "/" + secret.Key
		switch {
		case !secret.Encrypted:
			if params.runs(status.SecretIsNotEncrypted) {
				report.record(params, Finding{
					Key:     identity,
					Code:    status.SecretIsNotEncrypted,
					Message: "stored value is not encrypted",
				})
			}
		case secret.AlgorithmMismatch:
			if params.runs(status.SecretAlgorithmMismatch) {
				report.record(params, Finding{
					Key:     identity,
					Code:    status.SecretAlgorithmMismatch,
					Message: "stored under a different algorithm than the configured cipher",
				})
			}
		}
		if params.runs(status.SecretIsNotReferenced) {
			ref := storeRef{group: strings.ToLower(secret.Group), key: secret.Key}
			if !referenced[ref] {
				report.record(params, Finding{
					Key:     identity,
					Code:    status.SecretIsNotReferenced,
					Message: "stored value is never referenced by any environment",
				})
			}
		}
	}

	if params.runs(status.PublicKeyIsMissing) {
		groups, err := manager.GroupsMissingPublicKey()
		if err != nil {
			return fmt.Errorf("reading public keys: %w", err)
		}
		for _, group := range groups {
			report.record(params, Finding{
				Key:     group,
				Code:    status.PublicKeyIsMissing,
				Message: "the group has stored secrets but no public key",
			})
		}
	}
	return nil
}

// addKeypairFindings reports the keypair-health checks — an invalid or unavailable
// private key — reading the group keypairs without exposing key material. Each
// finding is gated by its own selection so one keypair check can run without the
// other.
func addKeypairFindings(report *Report, params Params, manager *secrets.Manager) error {
	keypairs, err := manager.Keypairs()
	if err != nil {
		return fmt.Errorf("reading keypairs: %w", err)
	}
	for _, keypair := range keypairs {
		finding, ok := keypairFinding(keypair)
		if !ok || !params.runs(finding.Code) {
			continue
		}
		report.record(params, finding)
	}
	return nil
}

// keypairFinding maps a keypair's private-key status onto an ungraded finding: an
// invalid key and an unavailable key each carry their code, while a valid key
// produces no finding. The report grades the returned finding by its code.
func keypairFinding(keypair secrets.KeypairMetadata) (Finding, bool) {
	switch keypair.PrivateKeyStatus {
	case secrets.PrivateKeyInvalid:
		return Finding{
			Key:     keypair.Group,
			Code:    status.PrivateKeyIsInvalid,
			Message: "the private key for this group is malformed or mismatched",
		}, true
	case secrets.PrivateKeyNotAvailable:
		return Finding{
			Key:     keypair.Group,
			Code:    status.PrivateKeyIsUnavailable,
			Message: "no private key for this group in this context",
		}, true
	default:
		return Finding{}, false
	}
}

// Sort orders the report's findings deterministically for stable output and
// scripting: errors before warnings, then by check group and code, project,
// environment, and key. It sorts in place and returns the report for chaining.
func (r *Report) Sort() *Report {
	sort.SliceStable(r.Findings, func(i, j int) bool {
		return less(r.Findings[i], r.Findings[j])
	})
	return r
}

// less orders two findings: error severity first, then by the check group and
// code that produced them, then project, environment, and key, so the output is
// deterministic across runs and findings from the same check cluster together.
func less(a, b Finding) bool {
	if a.Severity != b.Severity {
		return a.Severity == SeverityError
	}
	if ga, gb := groupRank(a.Code), groupRank(b.Code); ga != gb {
		return ga < gb
	}
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	if a.Project != b.Project {
		return a.Project < b.Project
	}
	if a.Environment != b.Environment {
		return a.Environment < b.Environment
	}
	return a.Key < b.Key
}
