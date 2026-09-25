package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/internal/core"
	engine "github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/utils/filex"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// actionParams are the inputs to the emit action.
type actionParams struct {
	// Project is the project whose environment is resolved.
	Project string
	// Target selects the output format.
	Target engine.Target
	// Name overrides the base for k8s resource names; empty defaults to Project.
	// The render appends the slice suffix (-config, -secrets, -config-secrets).
	Name string
	// IncludeSecrets emits the secret-derived values.
	IncludeSecrets bool
	// IncludeConfig emits the plain (non-secret) values.
	IncludeConfig bool
	// Key overrides the k8s-bundle data key (and mounted filename); empty defaults
	// to config.json, or secrets.json for a secrets-only bundle. Its extension
	// picks the body format.
	Key string
	// OutputPath writes the render to a file instead of stdout when non-empty.
	OutputPath string
}

// execute is the imperative shell: resolve the project, reveal and classify every
// value through the diagnostic reveal path, fail closed if any value is
// unresolved, then render the result to the chosen target. It buffers the render
// so a file target commits atomically and nothing is written when resolution
// fails. When a file target's output carries secret material it warns through
// the printer, since the written file is unencrypted and must not be committed.
func execute(
	p actionParams, in *core.Input, stdout io.Writer, pr *printer.Printer,
) error {
	// The slice flags are additive filters, so selecting neither is the same as
	// selecting both — emit everything. Normalizing here keeps the whole action
	// (render and the secret-file warning) reading one consistent selection.
	if !p.IncludeSecrets && !p.IncludeConfig {
		p.IncludeSecrets, p.IncludeConfig = true, true
	}

	// A k8s resource name defaults to the project, with the slice suffix appended
	// so one project yields app-config / app-secrets without a flag. An explicit
	// --name is honored verbatim instead.
	nameBase := p.Name
	exactName := p.Name != ""
	if nameBase == "" {
		nameBase = p.Project
	}

	// A k8s-bundle data key (and mounted filename) defaults per slice, so the
	// common case needs no --key: config.json, or secrets.json for a secrets-only
	// bundle.
	key := p.Key
	if p.Target == engine.TargetK8sBundle && key == "" {
		key = defaultBundleKey(p.IncludeSecrets, p.IncludeConfig)
	}

	// Fail fast, before any decryption, if the output directory is missing —
	// emit writes the named file but does not create directories, so a bad path is
	// surfaced clearly rather than as a temp-file error after the work is done.
	if err := checkOutputDir(p.OutputPath); err != nil {
		return err
	}

	resolved, err := core.ResolveProject(in, p.Project)
	if err != nil {
		return err
	}

	// Reveal and classify every winning value. Explain never aborts on a per-key
	// failure — it carries the status instead — so emit enforces its own
	// fail-closed contract in entriesFromExplanation below.
	explanation, err := resolved.Envmerge.Explain(env.ExplainParams{Reveal: true})
	if err != nil {
		return err
	}

	entries, err := entriesFromExplanation(explanation)
	if err != nil {
		return err
	}

	// Buffer the render so no partial output escapes on a formatting error and a
	// file target is committed atomically.
	var buffer bytes.Buffer
	if err := engine.Render(&buffer, entries, engine.Params{
		Target:         p.Target,
		NameBase:       nameBase,
		ExactName:      exactName,
		IncludeSecrets: p.IncludeSecrets,
		IncludeConfig:  p.IncludeConfig,
		Key:            key,
	}); err != nil {
		return err
	}

	// Emitting to stdout stays quiet so the manifest can be piped cleanly; the
	// stderr confirmation is reserved for a file target.
	if p.OutputPath == "" {
		_, err := stdout.Write(buffer.Bytes())
		return err
	}

	// The render may carry plaintext secrets, so persist it with private
	// permissions rather than the world-readable default.
	if err := filex.WriteAtomicPrivate(p.OutputPath, buffer.Bytes()); err != nil {
		return err
	}
	// Confirm where the file landed as normal output — the summary, then (for a
	// k8s-bundle) the docs pointer in the same line. When it carries secret material
	// — the values are unencrypted (plaintext, or base64 in a Secret) — add a
	// blank line and a stderr caution not to commit it.
	block := wroteSummary(p)
	if p.Target == engine.TargetK8sBundle {
		block += " Read the docs to learn how to use the file in a Kubernetes volume."
	}
	if err := pr.LogMessage(block); err != nil {
		return err
	}
	if outputCarriesSecrets(p, entries) {
		if err := pr.LogBlank(); err != nil {
			return err
		}
		return pr.LogWarning("the output carries unencrypted secret values; do not commit it")
	}
	return nil
}

// wroteSummary is the one-line confirmation printed after a file write: which
// project's slice was written, and to which file.
func wroteSummary(p actionParams) string {
	slice := "config and secrets"
	switch {
	case p.IncludeSecrets && !p.IncludeConfig:
		slice = "secrets"
	case !p.IncludeSecrets && p.IncludeConfig:
		slice = "config"
	}
	return fmt.Sprintf("Wrote %s %s to %s.", p.Project, slice, p.OutputPath)
}

// checkOutputDir reports an error when the file's parent directory does not
// exist, so emit refuses to write into a missing path rather than creating
// directories the user did not ask for. An empty path (stdout) always passes.
func checkOutputDir(outputPath string) error {
	if outputPath == "" {
		return nil
	}
	dir := filepath.Dir(outputPath)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("output directory %s does not exist", dir)
	}
	return nil
}

// defaultBundleKey picks the default k8s-bundle data key from the selected
// slice: a secrets-only bundle defaults to secrets.json, and config-only or a
// merged bundle to config.json. The .json extension makes the body JSON.
func defaultBundleKey(includeSecrets, includeConfig bool) string {
	if includeSecrets && !includeConfig {
		return "secrets.json"
	}
	return "config.json"
}

// outputCarriesSecrets reports whether the rendered file will contain
// secret-derived values, so the caller can warn that it must not be committed.
// The file carries secrets only when the secret slice was selected and the
// environment actually holds a secret-derived value; a config-only render never
// triggers the warning.
func outputCarriesSecrets(p actionParams, entries []engine.Entry) bool {
	if !p.IncludeSecrets {
		return false
	}
	for i := range entries {
		if entries[i].Secret {
			return true
		}
	}
	return false
}

// entriesFromExplanation converts a revealed explanation into emit entries,
// enforcing emit's fail-closed contract: if any value did not resolve to
// plaintext it returns an error naming every unresolved key and no entries, so
// the caller emits nothing. A value classified as a secret reference becomes a
// secret-derived entry, which the Kubernetes split routes into a Secret.
func entriesFromExplanation(explanation *env.Explanation) ([]engine.Entry, error) {
	entries := make([]engine.Entry, 0, len(explanation.Entries))
	var unresolved []string
	for i := range explanation.Entries {
		e := &explanation.Entries[i]
		if !e.Resolution.HasResolved {
			unresolved = append(unresolved, e.Key)
			continue
		}
		entries = append(entries, engine.Entry{
			Key:    e.Key,
			Value:  e.Resolution.Resolved,
			Secret: e.Resolution.Kind == env.KindSecretReference,
		})
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return nil, fmt.Errorf(
			"cannot emit: %d value(s) did not resolve: %s",
			len(unresolved), strings.Join(unresolved, ", "),
		)
	}
	return entries, nil
}
