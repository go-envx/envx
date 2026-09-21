package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/fixtures"
	"github.com/go-envx/envx/app/internal/secrets"
	"github.com/go-envx/envx/app/internal/status"
	engine "github.com/go-envx/envx/app/internal/validate"
)

// executeManifest runs the validate action over the manifest at path.
func executeManifest(t *testing.T, path string, p actionParams) engine.Report {
	t.Helper()
	report, err := execute(p, &config.Input{ConfigPath: &path})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return report
}

// findFinding returns the first finding carrying the given status code, and
// whether one was present.
func findFinding(report engine.Report, code string) (engine.Finding, bool) {
	for _, f := range report.Findings {
		if f.Code == code {
			return f, true
		}
	}
	return engine.Finding{}, false
}

// TestExecuteCleanWorkspacePasses verifies a workspace with no store and no
// references produces no findings and does not fail.
func TestExecuteCleanWorkspacePasses(t *testing.T) {
	t.Parallel()

	report := executeManifest(t, fixtures.Manifest("basic"), actionParams{})
	if len(report.Findings) != 0 {
		t.Errorf("Findings = %d, want 0: %+v", len(report.Findings), report.Findings)
	}
	if report.Failed {
		t.Error("clean workspace must not fail")
	}
}

// TestExecuteDanglingReferenceFails verifies a dangling reference fails the run
// and is listed alongside the store-level findings the per-environment view
// cannot produce.
func TestExecuteDanglingReferenceFails(t *testing.T) {
	t.Parallel()

	path := fixtures.Testdata("resolve", "dangling-reference", "envx.yaml")
	report := executeManifest(t, path, actionParams{})

	if !report.Failed {
		t.Fatal("dangling reference must fail the run")
	}

	ref, ok := findFinding(report, status.SecretReferenceNotFound)
	if !ok {
		t.Fatalf("no reference finding: %+v", report.Findings)
	}
	if ref.Severity != engine.SeverityError || ref.Key != "PASSWORD" {
		t.Errorf("reference finding = %+v, want error on PASSWORD", ref)
	}

	// The store holds a plaintext value no environment references, so both a
	// plaintext error and an orphan error surface without a per-environment view.
	plaintext, ok := findFinding(report, status.SecretIsNotEncrypted)
	if !ok || plaintext.Severity != engine.SeverityError {
		t.Errorf("plaintext finding = %+v (present=%v), want an error", plaintext, ok)
	}
	orphan, ok := findFinding(report, status.SecretIsNotReferenced)
	if !ok || orphan.Severity != engine.SeverityError {
		t.Errorf("orphan finding = %+v (present=%v), want an error", orphan, ok)
	}
}

// TestExecuteStrictFailsOnWarningsOnly verifies a workspace whose only finding is
// a warning passes by default but fails under --strict.
func TestExecuteStrictFailsOnWarningsOnly(t *testing.T) {
	t.Parallel()

	manifestPath := writeWarningOnlyWorkspace(t)

	relaxed := executeManifest(t, manifestPath, actionParams{Strict: false})
	if relaxed.Failed {
		t.Errorf("warning-only workspace must pass by default: %+v", relaxed.Findings)
	}
	if relaxed.Warnings == 0 || relaxed.Errors != 0 {
		t.Fatalf("want warnings only, got %d errors %d warnings: %+v",
			relaxed.Errors, relaxed.Warnings, relaxed.Findings)
	}

	strict := executeManifest(t, manifestPath, actionParams{Strict: true})
	if !strict.Failed {
		t.Error("warning-only workspace must fail under --strict")
	}
}

// TestExecuteSeverityConfigOverrides verifies the manifest's validate block
// changes a check's severity: promoting the unavailable-key warning to an error
// fails the run, while a second workspace confirms turning a check off drops it.
func TestExecuteSeverityConfigOverrides(t *testing.T) {
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
	report := executeManifest(t, promoted, actionParams{})
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
	off := executeManifest(t, silenced, actionParams{Strict: true})
	if off.Failed || len(off.Findings) != 0 {
		t.Errorf("turning the check off must drop it even under --strict: %+v", off)
	}
}

// TestExecuteBaseDeclaration verifies the base-declaration check is off by
// default — an environment overlay key its namespace base file never declares is
// tolerated — and surfaces PROPERTY_NOT_DECLARED_IN_BASE only once a workspace
// opts in through the validate block.
func TestExecuteBaseDeclaration(t *testing.T) {
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

	off := executeManifest(t, filepath.Join(dir, "envx.yaml"), actionParams{})
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

	report := executeManifest(t, filepath.Join(optedIn, "envx.yaml"), actionParams{})
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

// TestExecuteMissingPublicKey verifies a group with stored secrets but no public
// key surfaces PUBLIC_KEY_IS_MISSING, isolated by selecting only that check.
func TestExecuteMissingPublicKey(t *testing.T) {
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

	report := executeManifest(t, filepath.Join(dir, "envx.yaml"), actionParams{
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

// TestExecuteAlgorithmMismatchFromStore verifies a value stored under a
// non-configured algorithm surfaces SECRET_ALGORITHM_MISMATCH from the store scan.
func TestExecuteAlgorithmMismatchFromStore(t *testing.T) {
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

	report := executeManifest(t, filepath.Join(dir, "envx.yaml"), actionParams{
		Selected: map[string]bool{status.SecretAlgorithmMismatch: true},
	})

	finding, ok := findFinding(report, status.SecretAlgorithmMismatch)
	if !ok || finding.Code != status.SecretAlgorithmMismatch || finding.Key != "db/token" {
		t.Fatalf("algorithm finding = %+v (present=%v), want %s on db/token",
			finding, ok, status.SecretAlgorithmMismatch)
	}
}

// TestExecuteStoreOnlySelectionSkipsMerge verifies that selecting only a store
// check performs no environment merge: a workspace whose merge would fail still
// validates its plaintext store value, while a full run aborts on the broken
// merge.
func TestExecuteStoreOnlySelectionSkipsMerge(t *testing.T) {
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
	_, err := execute(actionParams{}, &config.Input{ConfigPath: &manifestPath})
	if err == nil {
		t.Fatal("a full run must fail on the malformed base file")
	}

	// Selecting only the offline plaintext check skips the merge entirely and still
	// reports the plaintext store value.
	report := executeManifest(t, manifestPath, actionParams{
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
	manager, err := config.NewSecretsManager(
		secrets.Params{
			SecretsPath:   secretsPath,
			KeysPath:      filepath.Join(dir, "envx.keys"),
			DefaultIndent: 2,
		},
		cipher.Params{Algorithm: cipher.Age},
	)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
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
