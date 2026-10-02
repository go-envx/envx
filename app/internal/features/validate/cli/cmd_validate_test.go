package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/shared/status"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/test/fixtures"
)

// mockValidateFactory implements Factory for unit testing.
type mockValidateFactory struct {
	err error
}

func (m *mockValidateFactory) ValidateService(
	configPath string,
) (*validate.Service, error) {
	if m.err != nil {
		return nil, m.err
	}
	return core.NewApp().ValidateService(configPath)
}

// executeValidate builds the validate command, executes it with args, and returns
// its captured stdout and stderr.
func executeValidate(
	t *testing.T, factory Factory, configPath string, args ...string,
) (stdout, stderr string, err error) {
	t.Helper()

	cmd := NewValidateCommand(factory)
	flags.Bind(cmd.PersistentFlags(), &flags.Config)
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", configPath}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// validateManifest runs the validate service over the manifest at path and
// returns the sorted report.
func validateManifest(
	t *testing.T, path string, params validate.ValidateParams,
) validate.Report {
	t.Helper()

	service, err := core.NewApp().ValidateService(path)
	if err != nil {
		t.Fatalf("ValidateService(): %v", err)
	}
	report, err := service.Validate(params)
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	report.Sort()
	return report
}

// findFinding returns the first finding carrying the given status code, and
// whether one was present.
func findFinding(report validate.Report, code string) (validate.Finding, bool) {
	for _, f := range report.Findings {
		if f.Code == code {
			return f, true
		}
	}
	return validate.Finding{}, false
}

// sampleReport is a mixed error/warning report used by the output tests.
func sampleReport() validate.Report {
	return validate.Report{
		Findings: []validate.Finding{
			{
				Severity: validate.SeverityError,
				Project:  "api", Environment: "production", Key: "PASSWORD",
				Code: status.SecretReferenceNotFound, Message: "no stored value for this reference",
			},
			{
				Severity: validate.SeverityWarning,
				Key:      "shared/unused", Code: status.SecretIsNotReferenced,
				Message: "stored value is never referenced by any environment",
			},
		},
		Errors: 1, Warnings: 1, Failed: true,
	}
}

// TestValidateCleanWorkspacePasses verifies a workspace with no store and no
// references produces no findings and does not fail.
func TestValidateCleanWorkspacePasses(t *testing.T) {
	t.Parallel()

	report := validateManifest(t, fixtures.Manifest("basic"), validate.ValidateParams{})
	if len(report.Findings) != 0 {
		t.Errorf("Findings = %d, want 0: %+v", len(report.Findings), report.Findings)
	}
	if report.Failed {
		t.Error("clean workspace must not fail")
	}
}

// TestValidateDanglingReferenceFails verifies a dangling reference fails the run
// and is listed alongside the store-level findings the per-environment view
// cannot produce.
func TestValidateDanglingReferenceFails(t *testing.T) {
	t.Parallel()

	path := fixtures.Testdata("resolve", "dangling-reference", "envx.yaml")
	report := validateManifest(t, path, validate.ValidateParams{})

	if !report.Failed {
		t.Fatal("dangling reference must fail the run")
	}

	ref, ok := findFinding(report, status.SecretReferenceNotFound)
	if !ok {
		t.Fatalf("no reference finding: %+v", report.Findings)
	}
	if ref.Severity != validate.SeverityError || ref.Key != "PASSWORD" {
		t.Errorf("reference finding = %+v, want error on PASSWORD", ref)
	}

	// The store holds a plaintext value no environment references, so both a
	// plaintext error and an orphan error surface without a per-environment view.
	plaintext, ok := findFinding(report, status.SecretIsNotEncrypted)
	if !ok || plaintext.Severity != validate.SeverityError {
		t.Errorf("plaintext finding = %+v (present=%v), want an error", plaintext, ok)
	}
	orphan, ok := findFinding(report, status.SecretIsNotReferenced)
	if !ok || orphan.Severity != validate.SeverityError {
		t.Errorf("orphan finding = %+v (present=%v), want an error", orphan, ok)
	}
}

// TestValidateStrictFailsOnWarningsOnly verifies a workspace whose only finding is
// a warning passes by default but fails under --strict.
func TestValidateStrictFailsOnWarningsOnly(t *testing.T) {
	t.Parallel()

	manifestPath := writeWarningOnlyWorkspace(t)

	relaxed := validateManifest(t, manifestPath, validate.ValidateParams{Strict: false})
	if relaxed.Failed {
		t.Errorf("warning-only workspace must pass by default: %+v", relaxed.Findings)
	}
	if relaxed.Warnings == 0 || relaxed.Errors != 0 {
		t.Fatalf("want warnings only, got %d errors %d warnings: %+v",
			relaxed.Errors, relaxed.Warnings, relaxed.Findings)
	}

	strict := validateManifest(t, manifestPath, validate.ValidateParams{Strict: true})
	if !strict.Failed {
		t.Error("warning-only workspace must fail under --strict")
	}
}

// TestValidateSeverityConfigOverrides verifies the manifest's validate block
// changes a check's severity: promoting the unavailable-key warning to an error
// fails the run, while a second workspace confirms turning a check off drops it.
func TestValidateSeverityConfigOverrides(t *testing.T) {
	t.Parallel()

	// Promote the unavailable-key warning to an error via the validate block.
	promoted := writeWarningOnlyWorkspace(t)
	writeFile(t, promoted,
		"environments: [development, production]\n"+
			"validate:\n"+
			"  private_key_is_unavailable: error\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	report := validateManifest(t, promoted, validate.ValidateParams{})
	if !report.Failed || report.Errors == 0 {
		t.Errorf("promoting the unavailable key to error must fail: %+v", report)
	}

	// Turn the same check off: the workspace then has no findings and passes.
	silenced := writeWarningOnlyWorkspace(t)
	writeFile(t, silenced,
		"environments: [development, production]\n"+
			"validate:\n"+
			"  private_key_is_unavailable: off\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	off := validateManifest(t, silenced, validate.ValidateParams{Strict: true})
	if off.Failed || len(off.Findings) != 0 {
		t.Errorf("turning the check off must drop it even under --strict: %+v", off)
	}
}

// TestValidateBaseDeclaration verifies the base-declaration check is off by
// default — an environment overlay key its namespace base file never declares is
// tolerated — and surfaces PROPERTY_NOT_DECLARED_IN_BASE only once a workspace
// opts in through the validate block.
func TestValidateBaseDeclaration(t *testing.T) {
	t.Parallel()

	// By default the check is off: the overlay-only key produces no finding.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "envx.yaml"),
		"environments: [production]\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	writeFile(t, filepath.Join(dir, "env", "app.yaml"), "shared: base-value\n")
	writeFile(t, filepath.Join(dir, "env", "app.production.yaml"), "only_in_overlay: x\n")

	off := validateManifest(t, filepath.Join(dir, "envx.yaml"), validate.ValidateParams{})
	if _, ok := findFinding(off, status.PropertyNotDeclaredInBase); ok {
		t.Errorf("base-declaration check must be off by default: %+v", off.Findings)
	}
	if off.Failed {
		t.Errorf("an overlay-only key must not fail by default: %+v", off.Findings)
	}

	// Opting in through the validate block surfaces the finding and fails the run.
	optedIn := t.TempDir()
	writeFile(t, filepath.Join(optedIn, "envx.yaml"),
		"environments: [production]\n"+
			"validate:\n"+
			"  property_not_declared_in_base: error\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	writeFile(t, filepath.Join(optedIn, "env", "app.yaml"), "shared: base-value\n")
	writeFile(t,
		filepath.Join(optedIn, "env", "app.production.yaml"), "only_in_overlay: x\n")

	report := validateManifest(
		t, filepath.Join(optedIn, "envx.yaml"), validate.ValidateParams{},
	)
	finding, ok := findFinding(report, status.PropertyNotDeclaredInBase)
	if !ok {
		t.Fatalf("no base-declaration finding after opt-in: %+v", report.Findings)
	}
	if finding.Code != status.PropertyNotDeclaredInBase ||
		finding.Key != "ONLY_IN_OVERLAY" {
		t.Errorf("finding = %+v, want base-declaration on ONLY_IN_OVERLAY", finding)
	}
	if !report.Failed {
		t.Error("an undeclared overlay key must fail once configured to error")
	}
}

// TestValidateMissingPublicKey verifies a group with stored secrets but no public
// key surfaces PUBLIC_KEY_IS_MISSING, isolated by selecting only that check.
func TestValidateMissingPublicKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "envx.yaml"),
		"environments: [production]\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	writeFile(t, filepath.Join(dir, "env", "app.yaml"), "name: app-value\n")
	writeFile(t, filepath.Join(dir, "secrets.yaml"),
		"secrets:\n  db:\n    password: encrypted-age:YWJj\n")

	report := validateManifest(t, filepath.Join(dir, "envx.yaml"), validate.ValidateParams{
		Selected: map[string]bool{status.PublicKeyIsMissing: true},
	})

	finding, ok := findFinding(report, status.PublicKeyIsMissing)
	if !ok || finding.Code != status.PublicKeyIsMissing || finding.Key != "db" {
		t.Fatalf("missing-public-key finding = %+v (present=%v), want %s on db",
			finding, ok, status.PublicKeyIsMissing)
	}
	// Only the selected check ran, so the orphaned encrypted value is not reported.
	if _, ok := findFinding(report, status.SecretIsNotReferenced); ok {
		t.Error("selecting only the missing-public-key check must not report orphans")
	}
}

// TestValidateAlgorithmMismatchFromStore verifies a value stored under a
// non-configured algorithm surfaces SECRET_ALGORITHM_MISMATCH from the store scan.
func TestValidateAlgorithmMismatchFromStore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "envx.yaml"),
		"environments: [production]\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	writeFile(t, filepath.Join(dir, "env", "app.yaml"), "name: app-value\n")
	// The default cipher is age; a nacl-box envelope is a mismatch.
	writeFile(t, filepath.Join(dir, "secrets.yaml"),
		"secrets:\n  db:\n    token: encrypted-nacl-box:YWJj\n")

	report := validateManifest(t, filepath.Join(dir, "envx.yaml"), validate.ValidateParams{
		Selected: map[string]bool{status.SecretAlgorithmMismatch: true},
	})

	finding, ok := findFinding(report, status.SecretAlgorithmMismatch)
	if !ok || finding.Code != status.SecretAlgorithmMismatch || finding.Key != "db/token" {
		t.Fatalf("algorithm finding = %+v (present=%v), want %s on db/token",
			finding, ok, status.SecretAlgorithmMismatch)
	}
}

// TestValidateStoreOnlySelectionSkipsMerge verifies that selecting only a store
// check performs no environment merge: a workspace whose merge would fail still
// validates its plaintext store value, while a full run aborts on the broken
// merge.
func TestValidateStoreOnlySelectionSkipsMerge(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "envx.yaml")
	writeFile(t, manifestPath,
		"environments: [production]\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	// A base file whose YAML cannot parse makes the environment merge fatal.
	writeFile(t, filepath.Join(dir, "env", "app.yaml"), "name: : : [unbalanced\n")
	writeFile(t, filepath.Join(dir, "secrets.yaml"),
		"secrets:\n  db:\n    leaked: just-plaintext\n")

	// A full run reaches the merge and aborts on the malformed base file.
	service, err := core.NewApp().ValidateService(manifestPath)
	if err != nil {
		t.Fatalf("ValidateService(): %v", err)
	}
	if _, err := service.Validate(validate.ValidateParams{}); err == nil {
		t.Fatal("a full run must fail on the malformed base file")
	}

	// Selecting only the offline plaintext check skips the merge entirely and still
	// reports the plaintext store value.
	report := validateManifest(t, manifestPath, validate.ValidateParams{
		Selected: map[string]bool{status.SecretIsNotEncrypted: true},
	})
	finding, ok := findFinding(report, status.SecretIsNotEncrypted)
	if !ok || finding.Key != "db/leaked" {
		t.Fatalf("plaintext finding = %+v (present=%v), want db/leaked", finding, ok)
	}
	if !report.Failed {
		t.Error("a plaintext store value must fail the run")
	}
}

// TestNewValidateCommandFailsOnErrors verifies the command renders the findings
// and returns the validation verdict when the graded report fails.
func TestNewValidateCommandFailsOnErrors(t *testing.T) {
	t.Parallel()

	path := fixtures.Testdata("resolve", "dangling-reference", "envx.yaml")
	stdout, _, err := executeValidate(t, &mockValidateFactory{}, path)
	if !errors.Is(err, errValidationFailed) {
		t.Fatalf("err = %v, want errValidationFailed", err)
	}
	if !strings.Contains(stdout, "SECRET_REFERENCE_NOT_FOUND") {
		t.Errorf("stdout = %q, want the reference finding", stdout)
	}
}

// TestNewValidateCommandCleanWorkspace verifies the command confirms a clean
// workspace and exits without error.
func TestNewValidateCommandCleanWorkspace(t *testing.T) {
	t.Parallel()

	stdout, _, err := executeValidate(
		t, &mockValidateFactory{}, fixtures.Manifest("basic"),
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "no validation problems found" {
		t.Errorf("stdout = %q, want the clean confirmation line", got)
	}
}

// TestNewValidateCommandSelectionFlag verifies a per-check flag runs only the
// named check.
func TestNewValidateCommandSelectionFlag(t *testing.T) {
	t.Parallel()

	path := fixtures.Testdata("resolve", "dangling-reference", "envx.yaml")
	stdout, _, err := executeValidate(
		t, &mockValidateFactory{}, path, "--secret-is-not-encrypted",
	)
	if !errors.Is(err, errValidationFailed) {
		t.Fatalf("err = %v, want errValidationFailed", err)
	}
	if !strings.Contains(stdout, status.SecretIsNotEncrypted) {
		t.Errorf("stdout = %q, want the plaintext finding", stdout)
	}
	if strings.Contains(stdout, status.SecretReferenceNotFound) {
		t.Errorf("stdout = %q, must not run the unselected reference check", stdout)
	}
}

// TestNewValidateCommandJSON verifies --output=json renders the machine-readable
// envelope.
func TestNewValidateCommandJSON(t *testing.T) {
	t.Parallel()

	path := fixtures.Testdata("resolve", "dangling-reference", "envx.yaml")
	stdout, _, err := executeValidate(
		t, &mockValidateFactory{}, path, "--output", "json",
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

// TestNewValidateCommandFactoryError verifies a factory failure is returned
// unchanged.
func TestNewValidateCommandFactoryError(t *testing.T) {
	t.Parallel()

	want := errors.New("factory failed")
	_, _, err := executeValidate(
		t, &mockValidateFactory{err: want}, fixtures.Manifest("basic"),
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestOutputValidateTable verifies the table lists each finding with its scope
// and a severity banner precedes it.
func TestOutputValidateTable(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputValidate(printer.NewPlain(&out, &errOut), sampleReport(), "")
	if err != nil {
		t.Fatalf("outputValidate(): %v", err)
	}

	body := out.String()
	for _, want := range []string{
		"SCOPE", "KEY", "STATUS", "MESSAGE",
		"api/production", "PASSWORD", "SECRET_REFERENCE_NOT_FOUND",
		"secrets", "shared/unused", "SECRET_IS_NOT_REFERENCED",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("table missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "KIND") {
		t.Errorf("table must not include the removed KIND column:\n%s", body)
	}

	banner := errOut.String()
	if !strings.Contains(banner, "1 error(s) found") {
		t.Errorf("banner missing error count:\n%s", banner)
	}
	if !strings.Contains(banner, "1 warning(s) found") {
		t.Errorf("banner missing warning count:\n%s", banner)
	}
}

// TestOutputValidateTableSeparatesVerdict verifies a failing table is followed by
// a blank line — so the process boundary's "validation failed" verdict stands
// apart — while a passing report with findings is not. Both streams share one
// buffer so the interleaved write order (table then separator) is asserted
// faithfully.
func TestOutputValidateTableSeparatesVerdict(t *testing.T) {
	t.Parallel()

	orphan := validate.Finding{
		Severity: validate.SeverityWarning,
		Key:      "shared/unused", Code: status.SecretIsNotReferenced,
		Message: "stored value is never referenced by any environment",
	}

	// A failing report ends with the table's final newline plus a separating
	// blank; a passing report ends with just the table's final newline.
	if got := outputCombined(t, validate.Report{
		Findings: []validate.Finding{orphan}, Errors: 1, Failed: true,
	}); !strings.HasSuffix(got, "\n\n") {
		t.Errorf("failing table must be followed by a blank line:\n%q", got)
	}
	if got := outputCombined(t, validate.Report{
		Findings: []validate.Finding{orphan}, Warnings: 1, Failed: false,
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
	if got := strings.TrimSpace(out.String()); got != "no validation problems found" {
		t.Errorf("output = %q, want the clean confirmation line", got)
	}
	if strings.Contains(out.String(), "SCOPE") {
		t.Error("clean report must not print a table header")
	}
}

// TestOutputValidateJSON verifies the JSON envelope carries the summary and a
// tagged findings array, omitting empty scope fields for a store-level finding.
func TestOutputValidateJSON(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputValidate(printer.NewPlain(&out, &errOut), sampleReport(), "json")
	if err != nil {
		t.Fatalf("outputValidate(): %v", err)
	}

	var decoded validateReportJSON
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out.String())
	}
	summary := decoded.Summary
	if !summary.Failed || summary.Errors != 1 || summary.Warnings != 1 {
		t.Errorf("summary = %+v, want failed 1 error 1 warning", summary)
	}
	if len(decoded.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(decoded.Findings))
	}

	// The store-level orphan finding omits project and environment.
	if strings.Contains(out.String(), `"project": ""`) {
		t.Errorf("empty project should be omitted:\n%s", out.String())
	}
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

// writeWarningOnlyWorkspace builds a temp workspace whose only finding is a
// warning: a group whose public key is present but whose private key is not
// available in this context (PRIVATE_KEY_IS_UNAVAILABLE, the one check that
// defaults to warn). It stores no secrets, so nothing is orphaned or plaintext,
// and its env file has no references. It returns the manifest path.
func writeWarningOnlyWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	secretsPath := filepath.Join(dir, "secrets.yaml")

	// A valid keypair keeps the store readable; "shared" is available and produces
	// no finding.
	manager, err := core.NewSecretsService(
		secretsPath,
		filepath.Join(dir, "envx.keys"),
		cipher.Params{Algorithm: cipher.Age},
		2,
	)
	if err != nil {
		t.Fatalf("NewSecretsService(): %v", err)
	}
	if _, err := manager.GenerateKeypair("shared"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}

	// Add a second group that declares a public key but has no private key in the
	// keys file, so validate reports it as an unavailable-key warning.
	addUnavailableGroup(t, secretsPath, "legacy")

	writeFile(t, filepath.Join(dir, "envx.yaml"),
		"environments: [development, production]\n"+
			"projects:\n"+
			"  api:\n"+
			"    includes: [env/app]\n")
	writeFile(t, filepath.Join(dir, "env", "app.yaml"), "name: app-value\n")

	return filepath.Join(dir, "envx.yaml")
}

// addUnavailableGroup appends a public-key entry for group to the store at path,
// reusing an existing group's public key value so the entry is well-formed. The
// keys file holds no private key for it, so validate reports it as unavailable.
func addUnavailableGroup(t *testing.T, path, group string) {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Duplicate the first public-key entry under the new group name.
		if strings.HasPrefix(trimmed, "shared:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "shared:"))
			lines = append(lines[:i+1], append([]string{"  " + group + ": " + value},
				lines[i+1:]...)...)
			break
		}
	}
	//nolint:gosec // G703: path is created inside this test's temporary directory.
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// writeFile writes content to path, creating parent directories.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
