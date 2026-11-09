package env

import (
	"errors"
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/core/workspace"
)

// explainFixture serves secrets, templates, and failures for diagnosis.
func explainFixture(t *testing.T, host ...string) *Service {
	t.Helper()
	namespaces := newFakeNamespaces().
		base(t, appInclude, `
			password: secret://db/password
			host: db.local
			url: postgres://{{HOST}}:{{PASSWORD}}
			dangling: secret://db/missing
			loop: "{{LOOP}}"
			undefined: "{{NOWHERE}}"
			escaped: \{{HOST}}
			hosts: [a, secret://db/password]
		`).
		overlay(t, appInclude, "development", "extra: overlay-only")
	opener := &fakeOpener{plaintext: map[string]string{"secret://db/password": "s3cr3t"}}
	return newService(t, testWorkspace(), Dependencies{
		Namespaces:   namespaces,
		HostEnv:      hostOf(host...),
		OpenResolver: opener.open,
	})
}

// explainEntries explains every key and indexes entries by key.
func explainEntries(t *testing.T, s *Service, params ExplainParams) map[string]Entry {
	t.Helper()
	result, err := s.Explain(params)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	keys := make([]string, 0, len(result.Entries))
	out := make(map[string]Entry, len(result.Entries))
	for i := range result.Entries {
		keys = append(keys, result.Entries[i].Key)
		out[result.Entries[i].Key] = result.Entries[i]
	}
	if !slices.IsSorted(keys) {
		t.Errorf("keys = %v, want sorted", keys)
	}
	if result.WorkspaceDir != "/ws" {
		t.Errorf("WorkspaceDir = %q, want /ws", result.WorkspaceDir)
	}
	return out
}

// TestExplainEvaluations verifies classification, masking, and findings.
func TestExplainEvaluations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// key is the explained key.
		key string
		// reveal requests plaintext.
		reveal bool
		// want is the expected evaluation, ignoring finding messages.
		want value.Evaluation
	}{
		{key: "HOST", want: value.Evaluation{Value: "db.local", Resolved: true}},
		{key: "PASSWORD", want: value.Evaluation{
			Kind: value.Secret, Value: "secret://db/password",
		}},
		{key: "PASSWORD", reveal: true, want: value.Evaluation{
			Kind: value.Secret, Value: "s3cr3t", Resolved: true,
		}},
		{key: "HOSTS", reveal: true, want: value.Evaluation{
			Kind: value.Secret, Value: "a,s3cr3t", Resolved: true,
		}},
		{key: "URL", want: value.Evaluation{Kind: value.Variable}},
		{key: "URL", reveal: true, want: value.Evaluation{
			Kind: value.Variable, Value: "postgres://db.local:s3cr3t", Resolved: true,
		}},
		{key: "ESCAPED", reveal: true, want: value.Evaluation{
			Value: "{{HOST}}", Resolved: true,
		}},
		{key: "DANGLING", want: value.Evaluation{
			Kind: value.Secret, Value: "secret://db/missing",
			Diagnostic: diagnostic.Finding{Code: diagnostic.SecretReferenceNotFound},
		}},
		{key: "LOOP", want: value.Evaluation{
			Kind:       value.Variable,
			Diagnostic: diagnostic.Finding{Code: diagnostic.CircularVariableReference},
		}},
		{key: "UNDEFINED", reveal: true, want: value.Evaluation{
			Kind:       value.Variable,
			Diagnostic: diagnostic.Finding{Code: diagnostic.UnresolvedVariableReference},
		}},
	}

	for _, tt := range tests {
		entries := explainEntries(t, explainFixture(t), ExplainParams{Reveal: tt.reveal})
		got := entries[tt.key].Evaluation
		if !tt.want.Diagnostic.IsZero() {
			def, _ := diagnostic.Lookup(tt.want.Diagnostic.Code)
			tt.want.Diagnostic.Level = def.Level
			tt.want.Diagnostic.Project = "app"
			tt.want.Diagnostic.Environment = "development"
			tt.want.Diagnostic.Key = tt.key
			tt.want.Diagnostic.Message = got.Diagnostic.Message
		}
		if got != tt.want {
			t.Errorf("%s reveal=%v = %+v, want %+v", tt.key, tt.reveal, got, tt.want)
		}
	}
}

// TestExplainEntryMetadata verifies raw items, provenance, base declaration,
// and secret derivation.
func TestExplainEntryMetadata(t *testing.T) {
	t.Parallel()

	entries := explainEntries(t, explainFixture(t), ExplainParams{})

	hosts := entries["HOSTS"]
	if hosts.Raw != "a,secret://db/password" ||
		!slices.Equal(hosts.Items, []string{"a", "secret://db/password"}) {
		t.Errorf("HOSTS raw = %q items = %v", hosts.Raw, hosts.Items)
	}
	if !entries["HOST"].DeclaredInBase || entries["EXTRA"].DeclaredInBase {
		t.Error("DeclaredInBase should be true for HOST and false for EXTRA")
	}
	if got := entries["EXTRA"].Origin.Winner.Path; got != appInclude+".development.yaml" {
		t.Errorf("EXTRA winner = %q", got)
	}
	for key, want := range map[string]bool{
		"PASSWORD": true, "URL": true, "HOSTS": true, "HOST": false, "LOOP": false,
	} {
		if entries[key].Secret != want {
			t.Errorf("%s Secret = %v, want %v", key, entries[key].Secret, want)
		}
	}
}

// TestExplainHostValues verifies host values are opaque, masked unless
// revealed, and stay secret when they override a secret.
func TestExplainHostValues(t *testing.T) {
	t.Parallel()

	s := explainFixture(t, "PASSWORD=from-host", "HOST=h", "HOST_ONLY=x")
	entries := explainEntries(t, s, ExplainParams{})
	password := entries["PASSWORD"]
	if password.Raw != "" || password.Evaluation.Value != "" ||
		password.Evaluation.Resolved || !password.Secret {
		t.Errorf("masked host entry = %+v", password)
	}
	if !slices.Equal(password.Items, []string{"secret://db/password"}) {
		t.Errorf("PASSWORD items = %v, want the replaced namespace items", password.Items)
	}
	if entries["HOST"].Secret || entries["HOST"].Raw != "" {
		t.Errorf("host override of config = %+v", entries["HOST"])
	}
	if !entries["URL"].Secret {
		t.Error("URL should derive from the host value standing in for a secret")
	}
	if _, ok := entries["HOST_ONLY"]; ok {
		t.Error("host-only key explained")
	}

	revealed := explainEntries(t, s, ExplainParams{Reveal: true})
	if got := revealed["PASSWORD"].Evaluation; got.Value != "from-host" || !got.Resolved {
		t.Errorf("revealed host evaluation = %+v", got)
	}

	overloaded := explainEntries(t, s, ExplainParams{
		Query: Query{Options: workspace.Settings{Overload: ptr(true)}},
	})
	if !overloaded["URL"].Secret {
		t.Error("URL should derive from the namespace secret under overload")
	}
}

// TestExplainFindings verifies an entry lists every distinct item finding,
// located, while its evaluation keeps the most severe.
func TestExplainFindings(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().base(t, appInclude, `
		mixed: [secret://db/plain, secret://db/missing, secret://db/missing]
		loop: "{{LOOP}}"
		host: db.local
	`)
	opener := &fakeOpener{codes: map[string]diagnostic.Code{
		"secret://db/plain": diagnostic.SecretIsNotEncrypted,
	}}
	s := newService(t, testWorkspace(), Dependencies{
		Namespaces:   namespaces,
		OpenResolver: opener.open,
	})

	entries := explainEntries(t, s, ExplainParams{})

	mixed := entries["MIXED"]
	var got []diagnostic.Code
	for _, found := range mixed.Findings {
		got = append(got, found.Code)
		if found.Project != "app" || found.Environment != "development" ||
			found.Key != "MIXED" {
			t.Errorf("finding not located: %+v", found)
		}
	}
	want := []diagnostic.Code{
		diagnostic.SecretIsNotEncrypted, diagnostic.SecretReferenceNotFound,
	}
	if !slices.Equal(got, want) {
		t.Errorf("MIXED findings = %v, want %v", got, want)
	}
	if mixed.Evaluation.Diagnostic != mixed.Findings[0] {
		t.Errorf("MIXED evaluation = %+v, want the first finding", mixed.Evaluation)
	}
	if loop := entries["LOOP"].Findings; len(loop) != 1 ||
		loop[0].Code != diagnostic.CircularVariableReference {
		t.Errorf("LOOP findings = %+v", loop)
	}
	if host := entries["HOST"].Findings; host != nil {
		t.Errorf("HOST findings = %+v, want none", host)
	}
}

// TestExplainKey verifies single-key selection is case-insensitive and a
// missing key fails.
func TestExplainKey(t *testing.T) {
	t.Parallel()

	s := explainFixture(t)
	result, err := s.Explain(ExplainParams{Key: "host"})
	if err != nil || len(result.Entries) != 1 || result.Entries[0].Key != "HOST" {
		t.Errorf("Explain(host) = %+v, %v", result, err)
	}
	if _, err := s.Explain(ExplainParams{Key: "nope"}); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Explain(missing key) error = %v, want ErrKeyNotFound", err)
	}
}
