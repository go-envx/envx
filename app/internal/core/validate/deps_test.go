package validate

import (
	"slices"

	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/secrets"
)

// fakeExplainer is an in-memory Explainer returning the same entries for a
// project in every environment.
type fakeExplainer struct {
	// entries holds each project's entries.
	entries map[string][]env.ExplainEntry
	// calls records every request.
	calls []env.ExplainParams
	// err, when set, fails every call.
	err error
}

// fakeInspector is an in-memory Inspector returning sections by scope.
type fakeInspector struct {
	// audit holds every section.
	audit secrets.Audit
	// scopes records every requested scope.
	scopes []secrets.AuditScope
	// err, when set, fails every call.
	err error
}

// Explain returns the project's entries with findings located, as env does.
func (f *fakeExplainer) Explain(params env.ExplainParams) (env.ExplainResult, error) {
	f.calls = append(f.calls, params)
	if f.err != nil {
		return env.ExplainResult{}, f.err
	}
	entries := slices.Clone(f.entries[params.Project])
	for i := range entries {
		entries[i].Findings = slices.Clone(entries[i].Findings)
		for j := range entries[i].Findings {
			found := &entries[i].Findings[j]
			found.Project = params.Project
			found.Environment = *params.Options.Env
			found.Key = entries[i].Key
		}
	}
	return env.ExplainResult{Entries: entries}, nil
}

// Audit returns only the selected sections, as the store does.
func (f *fakeInspector) Audit(scope secrets.AuditScope) (secrets.Audit, error) {
	f.scopes = append(f.scopes, scope)
	if f.err != nil {
		return secrets.Audit{}, f.err
	}
	var out secrets.Audit
	if scope.Secrets {
		out.Secrets = f.audit.Secrets
	}
	if scope.Keypairs {
		out.Keypairs = f.audit.Keypairs
	}
	if scope.MissingPublicKeys {
		out.GroupsMissingPublicKey = f.audit.GroupsMissingPublicKey
	}
	return out, nil
}
