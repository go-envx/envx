package validate

import (
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/core/workspace"
)

// fixture bundles a workspace with the validate fakes.
type fixture struct {
	// ws is the workspace configuration passed to New.
	ws workspace.Workspace
	// explainer is the fake environment explainer.
	explainer *fakeExplainer
	// inspector is the fake store inspector.
	inspector *fakeInspector
	// envOpens counts explainer opens.
	envOpens int
	// secretsOpens counts inspector opens.
	secretsOpens int
}

// newFixture returns a workspace declaring projects in development and
// production with no entries or stored values.
func newFixture(projects ...string) *fixture {
	ws := workspace.Workspace{
		Environments: []string{"development", "production"},
		Projects:     make(map[string]workspace.Project),
	}
	for _, name := range projects {
		ws.Projects[name] = workspace.Project{}
	}
	return &fixture{
		ws:        ws,
		explainer: &fakeExplainer{entries: make(map[string][]env.Entry)},
		inspector: &fakeInspector{},
	}
}

// service constructs a Service whose openers count their calls.
func (f *fixture) service(t *testing.T) *Service {
	t.Helper()
	service, err := New(f.ws, Dependencies{
		OpenEnv: func() (Explainer, error) {
			f.envOpens++
			return f.explainer, nil
		},
		OpenSecrets: func() (Inspector, error) {
			f.secretsOpens++
			return f.inspector, nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return service
}

// validate runs Validate and fails the test on error.
func (f *fixture) validate(t *testing.T, params ValidateParams) Report {
	t.Helper()
	report, err := f.service(t).Validate(params)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return report
}

// entry returns a base-declared entry with raw items and no finding.
func entry(key string, items ...string) env.Entry {
	return env.Entry{Key: key, Items: items, DeclaredInBase: true}
}

// failing returns a base-declared entry whose evaluation reports code.
func failing(key string, code diagnostic.Code, items ...string) env.Entry {
	e := entry(key, items...)
	def, _ := diagnostic.Lookup(code)
	found := diagnostic.Finding{Code: code, Level: def.Level, Message: "m"}
	e.Evaluation = value.Evaluation{Kind: value.Secret, Diagnostic: found}
	e.Findings = []diagnostic.Finding{found}
	return e
}

// stored returns an audited stored value.
func stored(group, key string, encrypted, mismatch bool) secrets.StoredSecret {
	return secrets.StoredSecret{
		Reference:         secrets.Reference{Group: group, Key: key},
		Encrypted:         encrypted,
		AlgorithmMismatch: mismatch,
	}
}

// codes returns the findings' codes in order.
func codes(findings []diagnostic.Finding) []diagnostic.Code {
	out := make([]diagnostic.Code, len(findings))
	for i, found := range findings {
		out[i] = found.Code
	}
	return out
}

// count returns how many findings carry code.
func count(findings []diagnostic.Finding, code diagnostic.Code) int {
	n := 0
	for _, found := range findings {
		if found.Code == code {
			n++
		}
	}
	return n
}
