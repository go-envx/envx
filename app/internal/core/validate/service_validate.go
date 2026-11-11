package validate

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

// ValidateParams selects checks and the verdict policy for one run.
type ValidateParams struct {
	// Strict fails the run on warnings as well as errors.
	Strict bool
	// Selected lists the checks to run; empty runs every check.
	Selected []diagnostic.Code
	// Options are setting overrides applied to every project environment.
	Options workspace.Settings
}

// Report is the graded outcome of one run; it contains no secret values.
type Report struct {
	// Findings lists every reported problem, errors first.
	Findings []diagnostic.Finding
	// Errors counts findings at error severity.
	Errors int
	// Warnings counts findings at warning severity.
	Warnings int
	// Failed reports any error, or any warning under strict.
	Failed bool
}

// run is the state of one Validate call.
type run struct {
	// params are the normalized parameters.
	params ValidateParams
	// report accumulates graded findings.
	report Report
	// seen records reported findings, ignoring messages.
	seen map[diagnostic.Finding]bool
	// referenced records every stored value some environment references.
	referenced map[secrets.Reference]bool
	// schemes records every external scheme some environment's winner uses.
	schemes map[string]bool
}

// Validate diagnoses every project in every declared environment, then audits
// the store, and grades the findings. Only the inspectors a selected check
// needs are opened, and the store audit reads only the selected sections.
func (s *Service) Validate(params ValidateParams) (Report, error) {
	params, err := params.normalized()
	if err != nil {
		return Report{}, err
	}
	r := &run{
		params:     params,
		seen:       make(map[diagnostic.Finding]bool),
		referenced: make(map[secrets.Reference]bool),
		schemes:    make(map[string]bool),
	}
	merge := slices.ContainsFunc(Checks(), func(c Check) bool {
		return c.RequiresMerge && params.runs(c.Code)
	})
	if merge {
		if err := s.diagnose(r); err != nil {
			return Report{}, err
		}
	}
	if err := s.checkProviders(r); err != nil {
		return Report{}, err
	}
	if err := s.audit(r); err != nil {
		return Report{}, err
	}
	// Errors first, then by group, code, project, environment, and key.
	groupRank := func(f diagnostic.Finding) int {
		return slices.Index(groupOrder, owner(f.Code))
	}
	slices.SortStableFunc(r.report.Findings, func(a, b diagnostic.Finding) int {
		return cmp.Or(
			cmp.Compare(b.Level.Rank(), a.Level.Rank()),
			cmp.Compare(groupRank(a), groupRank(b)),
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Project, b.Project),
			cmp.Compare(a.Environment, b.Environment),
			cmp.Compare(a.Key, b.Key),
		)
	})
	r.report.Failed = r.report.Errors > 0 || (params.Strict && r.report.Warnings > 0)
	return r.report, nil
}

// normalized rejects codes outside the catalog.
func (p ValidateParams) normalized() (ValidateParams, error) {
	for _, code := range p.Selected {
		if owner(code) == "" {
			return ValidateParams{}, fmt.Errorf("%w %q", ErrUnknownCheck, code)
		}
	}
	p.Selected = slices.Clone(p.Selected)
	return p, nil
}

// runs reports whether the check for code is selected.
func (p ValidateParams) runs(code diagnostic.Code) bool {
	return len(p.Selected) == 0 || slices.Contains(p.Selected, code)
}

// diagnose explains each project in each environment, recording resolution
// findings, the stored values the environments reference, and the external
// schemes their winners use. Store-owned codes are left to the audit, which
// reports them once per stored value or group instead of once per reference.
func (s *Service) diagnose(r *run) error {
	explainer, err := s.deps.OpenEnv()
	if err != nil {
		return err
	}
	for _, project := range s.projects {
		for _, environment := range s.environments {
			options := r.params.Options
			options.Env = &environment
			result, err := explainer.Explain(env.ExplainParams{
				Project: project,
				Options: options,
			})
			if err != nil {
				return fmt.Errorf(
					"diagnosing project %q environment %q: %w", project, environment, err,
				)
			}
			for i := range result.Entries {
				entry := &result.Entries[i]
				for _, item := range entry.Items {
					if ref, err := secrets.ParseReference(item, environment); err == nil {
						r.referenced[ref] = true
					}
					// A host winner's items were replaced, so it calls no provider.
					scheme, ok := external.SchemeOf(item)
					if ok && !entry.Origin.Winner.Host {
						r.schemes[scheme] = true
					}
				}
				if !entry.DeclaredInBase {
					s.record(r, diagnostic.Finding{
						Code:        diagnostic.PropertyNotDeclaredInBase,
						Project:     project,
						Environment: environment,
						Key:         entry.Key,
						Message:     "declared only in an environment overlay, not the base file",
					})
				}
				for _, found := range entry.Findings {
					if owner(found.Code) == GroupResolution {
						s.record(r, found)
					}
				}
			}
		}
	}
	return nil
}

// checkProviders records each referenced provider that cannot be called, once
// per provider, opening the inspector only when the check runs and some
// winner references an external provider.
func (s *Service) checkProviders(r *run) error {
	if !r.params.runs(diagnostic.ExternalProviderUnavailable) || len(r.schemes) == 0 {
		return nil
	}
	inspector, err := s.deps.OpenProviders()
	if err != nil {
		return err
	}
	for _, scheme := range slices.Sorted(maps.Keys(r.schemes)) {
		if err := inspector.CheckAvailable(scheme); err != nil {
			s.record(r, diagnostic.Finding{
				Code:    diagnostic.ExternalProviderUnavailable,
				Key:     scheme,
				Message: err.Error(),
			})
		}
	}
	return nil
}

// audit records the store findings for the selected sections, opening the
// inspector only when a store check is selected.
func (s *Service) audit(r *run) error {
	p := r.params
	scope := secrets.AuditScope{
		Secrets: p.runs(diagnostic.SecretIsNotEncrypted) ||
			p.runs(diagnostic.SecretAlgorithmMismatch) ||
			p.runs(diagnostic.SecretIsNotReferenced),
		Keypairs: p.runs(diagnostic.PrivateKeyIsInvalid) ||
			p.runs(diagnostic.PrivateKeyIsUnavailable),
		MissingPublicKeys: p.runs(diagnostic.PublicKeyIsMissing),
	}
	if scope == (secrets.AuditScope{}) {
		return nil
	}
	inspector, err := s.deps.OpenSecrets()
	if err != nil {
		return err
	}
	audit, err := inspector.Audit(scope)
	if err != nil {
		return fmt.Errorf("auditing secrets store: %w", err)
	}
	for _, stored := range audit.Secrets {
		key := stored.Reference.Group + "/" + stored.Reference.Key
		switch {
		case !stored.Encrypted:
			s.record(r, diagnostic.Finding{
				Code:    diagnostic.SecretIsNotEncrypted,
				Key:     key,
				Message: "stored value is not encrypted",
			})
		case stored.AlgorithmMismatch:
			s.record(r, diagnostic.Finding{
				Code:    diagnostic.SecretAlgorithmMismatch,
				Key:     key,
				Message: "stored under a different algorithm than the configured cipher",
			})
		}
		if !r.referenced[stored.Reference] {
			s.record(r, diagnostic.Finding{
				Code:    diagnostic.SecretIsNotReferenced,
				Key:     key,
				Message: "stored value is never referenced by any environment",
			})
		}
	}
	for _, group := range audit.GroupsMissingPublicKey {
		s.record(r, diagnostic.Finding{
			Code:    diagnostic.PublicKeyIsMissing,
			Key:     group,
			Message: "the group has stored secrets but no public key",
		})
	}
	for _, keypair := range audit.Keypairs {
		switch keypair.PrivateKeyStatus {
		case secrets.PrivateKeyInvalid:
			s.record(r, diagnostic.Finding{
				Code:    diagnostic.PrivateKeyIsInvalid,
				Key:     keypair.Group,
				Message: "the private key for this group is malformed or mismatched",
			})
		case secrets.PrivateKeyNotAvailable:
			s.record(r, diagnostic.Finding{
				Code:    diagnostic.PrivateKeyIsUnavailable,
				Key:     keypair.Group,
				Message: "no private key for this group in this context",
			})
		}
	}
	return nil
}

// record grades a selected finding by its code and adds it once per code and
// subject, dropping codes configured off.
func (s *Service) record(r *run, found diagnostic.Finding) {
	if !r.params.runs(found.Code) {
		return
	}
	level, ok := s.levels[found.Code]
	if !ok {
		def, _ := diagnostic.Lookup(found.Code)
		level = def.Level
	}
	if level == severity.None {
		return
	}
	found.Level = level
	id := found
	id.Message = ""
	if r.seen[id] {
		return
	}
	r.seen[id] = true
	r.report.Findings = append(r.report.Findings, found)
	switch level {
	case severity.Error:
		r.report.Errors++
	case severity.Warn:
		r.report.Warnings++
	case severity.None, severity.OK:
	}
}
