package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/validate"
	"github.com/go-envx/envx/app/internal/core/workspace"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/printer"
	"github.com/go-envx/envx/app/internal/lib/severity"
	"github.com/go-envx/envx/app/internal/testutil"
)

// testConfig is the manifest path every command test passes with --config.
const testConfig = "/ws/envx.yaml"

// fakeFactory builds a validate service for a one-project workspace whose
// explainer and inspector are the factory itself.
type fakeFactory struct {
	// configPath records the manifest path the command requested.
	configPath string
	// err, when set, is returned instead of a service.
	err error
	// audit is returned by every store audit.
	audit secrets.Audit
	// explains records every explain request.
	explains []env.ExplainParams
	// scopes records every audit scope.
	scopes []secrets.AuditScope
}

// TestMain keeps ambient ENVX_* settings out of the command's flags.
func TestMain(m *testing.M) {
	testutil.UnsetEnvPrefix("ENVX_")
	os.Exit(m.Run())
}

// NewValidateService implements Factory.
func (f *fakeFactory) NewValidateService(configPath string) (*validate.Service, error) {
	f.configPath = configPath
	if f.err != nil {
		return nil, f.err
	}
	ws := workspace.Workspace{
		Environments: []string{"production"},
		Projects: map[string]workspace.Project{
			"api": {Includes: []string{"env/app"}},
		},
	}
	return validate.New(ws, validate.Dependencies{
		OpenEnv: func() (validate.Explainer, error) {
			return f, nil
		},
		OpenSecrets: func() (validate.Inspector, error) {
			return f, nil
		},
	})
}

// Explain implements validate.Explainer with no entries.
func (f *fakeFactory) Explain(params env.ExplainParams) (env.ExplainResult, error) {
	f.explains = append(f.explains, params)
	return env.ExplainResult{Project: params.Project}, nil
}

// Audit implements validate.Inspector.
func (f *fakeFactory) Audit(scope secrets.AuditScope) (secrets.Audit, error) {
	f.scopes = append(f.scopes, scope)
	return f.audit, nil
}

// plaintextAudit holds one unencrypted, unreferenced stored value.
func plaintextAudit() secrets.Audit {
	return secrets.Audit{
		Secrets: []secrets.StoredSecret{{
			Reference: secrets.Reference{Group: "db", Key: "leaked"},
		}},
	}
}

// executeValidate builds the validate command, executes it with args, and returns
// its captured stdout.
func executeValidate(
	t *testing.T, factory Factory, args ...string,
) (stdout string, err error) {
	t.Helper()

	cmd := NewValidateCommand(factory)
	workspacecli.BindConfig(cmd)
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", testConfig}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), err
}

// assertGolden compares got with the named file under testdata.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	want, err := filex.Read(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}

// sampleReport is a mixed error/warning report used by the output tests.
func sampleReport() validate.Report {
	return validate.Report{
		Findings: []diagnostic.Finding{
			{
				Level:   severity.Error,
				Project: "api", Environment: "production", Key: "PASSWORD",
				Code:    diagnostic.SecretReferenceNotFound,
				Message: "no stored value for this reference",
			},
			{
				Level: severity.Warn,
				Key:   "shared/unused", Code: diagnostic.SecretIsNotReferenced,
				Message: "stored value is never referenced by any environment",
			},
		},
		Errors: 1, Warnings: 1, Failed: true,
	}
}

// TestValidateCommandCleanWorkspace verifies --config reaches the factory and a
// clean workspace is confirmed without error.
func TestValidateCommandCleanWorkspace(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{}
	stdout, err := executeValidate(t, factory)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if factory.configPath != testConfig {
		t.Errorf("factory configPath = %q, want %q", factory.configPath, testConfig)
	}
	if got := strings.TrimSpace(stdout); got != "no validation problems found" {
		t.Errorf("stdout = %q, want the clean confirmation line", got)
	}
}

// TestValidateCommandFailsOnErrors verifies the command renders the findings
// and returns the verdict when the report fails.
func TestValidateCommandFailsOnErrors(t *testing.T) {
	t.Parallel()

	stdout, err := executeValidate(t, &fakeFactory{audit: plaintextAudit()})
	if !errors.Is(err, errValidationFailed) {
		t.Fatalf("err = %v, want errValidationFailed", err)
	}
	if !strings.Contains(stdout, string(diagnostic.SecretIsNotEncrypted)) {
		t.Errorf("stdout = %q, want the plaintext finding", stdout)
	}
}

// TestValidateCommandStrict verifies --strict fails a warning-only report.
func TestValidateCommandStrict(t *testing.T) {
	t.Parallel()

	warnOnly := secrets.Audit{
		Keypairs: []secrets.KeypairMetadata{{
			Group:            "legacy",
			PrivateKeyStatus: secrets.PrivateKeyNotAvailable,
		}},
	}
	if _, err := executeValidate(t, &fakeFactory{audit: warnOnly}); err != nil {
		t.Errorf("without --strict: err = %v, want nil", err)
	}
	_, err := executeValidate(t, &fakeFactory{audit: warnOnly}, "--strict")
	if !errors.Is(err, errValidationFailed) {
		t.Errorf("with --strict: err = %v, want errValidationFailed", err)
	}
}

// TestValidateCommandSelectionFlag verifies a per-check flag runs only that
// check: no environment is explained and only its store section is audited.
func TestValidateCommandSelectionFlag(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{audit: plaintextAudit()}
	stdout, err := executeValidate(t, factory, "--secret-is-not-encrypted")
	if !errors.Is(err, errValidationFailed) {
		t.Fatalf("err = %v, want errValidationFailed", err)
	}
	if len(factory.explains) != 0 {
		t.Errorf("explained %d environments, want none", len(factory.explains))
	}
	want := []secrets.AuditScope{{Secrets: true}}
	if !slices.Equal(factory.scopes, want) {
		t.Errorf("audit scopes = %+v, want %+v", factory.scopes, want)
	}
	if strings.Contains(stdout, string(diagnostic.SecretIsNotReferenced)) {
		t.Errorf("stdout = %q, must not report the unselected orphan check", stdout)
	}
}

// TestValidateCommandSettings verifies setting flags reach every explain
// request and that --env is not accepted.
func TestValidateCommandSettings(t *testing.T) {
	t.Parallel()

	factory := &fakeFactory{}
	if _, err := executeValidate(t, factory, "--prefix", "app"); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(factory.explains) == 0 {
		t.Fatal("no environment was explained")
	}
	if got := factory.explains[0].Options.Prefix; got == nil || *got != "app" {
		t.Errorf("Options.Prefix = %v, want app", got)
	}

	_, err := executeValidate(t, &fakeFactory{}, "--env", "production")
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --env") {
		t.Errorf("err = %v, want an unknown --env flag error", err)
	}
}

// TestValidateCommandJSON verifies --output=json renders the machine-readable
// envelope.
func TestValidateCommandJSON(t *testing.T) {
	t.Parallel()

	stdout, err := executeValidate(
		t, &fakeFactory{audit: plaintextAudit()}, "--output", "json",
	)
	if !errors.Is(err, errValidationFailed) {
		t.Fatalf("err = %v, want errValidationFailed", err)
	}

	var decoded validateReportJSON
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if !decoded.Summary.Failed || decoded.Summary.Errors == 0 {
		t.Errorf("summary = %+v, want a failed run with errors", decoded.Summary)
	}
}

// TestValidateCommandFactoryError verifies a factory failure is returned
// unchanged.
func TestValidateCommandFactoryError(t *testing.T) {
	t.Parallel()

	want := errors.New("factory failed")
	_, err := executeValidate(t, &fakeFactory{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestOutputValidateTable verifies the table against its golden and that a
// severity banner is written to stderr.
func TestOutputValidateTable(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputValidate(printer.NewPlain(&out, &errOut), sampleReport(), "")
	if err != nil {
		t.Fatalf("outputValidate(): %v", err)
	}
	assertGolden(t, "validate.txt", out.Bytes())

	banner := errOut.String()
	for _, want := range []string{"1 error(s) found", "1 warning(s) found"} {
		if !strings.Contains(banner, want) {
			t.Errorf("banner missing %q:\n%s", want, banner)
		}
	}
}

// TestOutputValidateTableSeparatesVerdict verifies a failing table is followed by
// a blank line — so the process boundary's "validation failed" verdict stands
// apart — while a passing report with findings is not. Both streams share one
// buffer so the interleaved write order (table then separator) is asserted
// faithfully.
func TestOutputValidateTableSeparatesVerdict(t *testing.T) {
	t.Parallel()

	orphan := diagnostic.Finding{
		Level: severity.Warn,
		Key:   "shared/unused", Code: diagnostic.SecretIsNotReferenced,
		Message: "stored value is never referenced by any environment",
	}

	// A failing report ends with the table's final newline plus a separating
	// blank; a passing report ends with just the table's final newline.
	if got := outputCombined(t, validate.Report{
		Findings: []diagnostic.Finding{orphan}, Errors: 1, Failed: true,
	}); !strings.HasSuffix(got, "\n\n") {
		t.Errorf("failing table must be followed by a blank line:\n%q", got)
	}
	if got := outputCombined(t, validate.Report{
		Findings: []diagnostic.Finding{orphan}, Warnings: 1, Failed: false,
	}); strings.HasSuffix(got, "\n\n") {
		t.Errorf("passing table must not add a trailing blank line:\n%q", got)
	}
}

// outputCombined renders report with a single buffer behind both streams so the
// terminal-visible ordering of stdout and stderr writes is preserved, and returns
// what a viewer would see.
func outputCombined(t *testing.T, report validate.Report) string {
	t.Helper()

	var combined bytes.Buffer
	if err := outputValidate(
		printer.NewPlain(&combined, &combined), report, "",
	); err != nil {
		t.Fatalf("outputValidate(): %v", err)
	}
	return combined.String()
}

// TestOutputValidateTableClean verifies a report with no findings prints a single
// confirmation line and no table.
func TestOutputValidateTableClean(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputValidate(printer.NewPlain(&out, &errOut), validate.Report{}, "table")
	if err != nil {
		t.Fatalf("outputValidate(): %v", err)
	}
	if got := out.String(); got != "no validation problems found\n" {
		t.Errorf("output = %q, want only the clean confirmation line", got)
	}
}

// TestOutputValidateJSON verifies the JSON envelope against its golden.
func TestOutputValidateJSON(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputValidate(printer.NewPlain(&out, &errOut), sampleReport(), "json")
	if err != nil {
		t.Fatalf("outputValidate(): %v", err)
	}
	assertGolden(t, "validate.json", out.Bytes())
}

// TestOutputValidateInvalidFormatFails verifies an unrecognized format is
// rejected loudly.
func TestOutputValidateInvalidFormatFails(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputValidate(printer.NewPlain(&out, &errOut), sampleReport(), "jsonn")
	if err == nil {
		t.Fatal("expected an error for an invalid format")
	}
}
