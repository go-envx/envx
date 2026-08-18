package explain

import (
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
)

// actionParams are the inputs to the explain action.
type actionParams struct {
	// Project is the project name to resolve.
	Project string
	// Key is the env-var key to look up (case-insensitive).
	// An empty string means "explain all keys".
	Key string
	// Reveal materializes resolved plaintext in a RESOLVED column.
	Reveal bool
	// Absolute renders source paths absolutely instead of relative to envx.yaml.
	Absolute bool
}

// actionResult is the data the explain action returns.
type actionResult struct {
	// Entries is the per-key explanation rows.
	Entries []actionResultEntry
	// Summary counts the incomplete outcomes across every entry.
	Summary envmerge.ExplanationSummary
}

// actionResultEntry is one row of explain output.
type actionResultEntry struct {
	// Key is the resolved env-var key (uppercased).
	Key string
	// Literal is the pre-resolution value written in the source file.
	Literal string
	// Source is the file that provided the resolved value.
	Source string
	// SourceKey is the original key in the source file that provided the value.
	SourceKey string
	// Shadowed is the list of files that were overridden by the resolved value.
	Shadowed []string
	// Resolution is the dry-run resolution outcome for the key, carrying its
	// kind, status, and any materialized plaintext.
	Resolution envmerge.Resolution
}

// execute is the imperative shell: resolve the project configuration and run the
// non-aborting diagnostic explanation through the constructed manager. Secret
// status is always computed; plaintext is materialized only under Reveal.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// resolve the input config
	resolved, err := config.ResolveProject(in, p.Project)
	if err != nil {
		return actionResult{}, err
	}

	// diagnose the selected keys; a per-key resolution failure is carried by its
	// status rather than aborting the command.
	explanation, err := resolved.Envmerge.Explain(envmerge.ExplainParams{
		Key:    p.Key,
		Reveal: p.Reveal,
	})
	if err != nil {
		return actionResult{}, err
	}

	return buildResult(explanation, resolved.WorkspaceDir(), p), nil
}

// buildResult maps a diagnostic explanation into presentation rows, converting
// each origin's absolute source paths for display and carrying the summary
// envmerge already computed.
func buildResult(
	explanation *envmerge.Explanation, workspaceDir string, p actionParams,
) actionResult {
	entries := make([]actionResultEntry, 0, len(explanation.Entries))
	for i := range explanation.Entries {
		e := &explanation.Entries[i]
		entries = append(entries, actionResultEntry{
			Key:        e.Key,
			Literal:    e.Literal,
			Source:     sourcePath(workspaceDir, e.Origin.Winner.File, p.Absolute),
			SourceKey:  e.Origin.Winner.Key,
			Shadowed:   shadowedFiles(e.Origin, workspaceDir, p.Absolute),
			Resolution: e.Resolution,
		})
	}
	return actionResult{Entries: entries, Summary: explanation.Summary}
}

// sourcePath renders one source file relative to the workspace root by default,
// or absolutely when requested. A path outside the workspace, or a failed
// relativization, falls back to the absolute path so a "../" escape never
// appears as a displayed relative path.
func sourcePath(workspaceDir, source string, absolute bool) string {
	if absolute || workspaceDir == "" {
		return source
	}
	rel, err := filepath.Rel(workspaceDir, source)
	if err != nil || rel == ".." ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return source
	}
	return rel
}

// shadowedFiles collects the files an origin overrode, in merge order, rendered
// in the same relative-or-absolute style as the winning source.
func shadowedFiles(
	origin envmerge.Origin, workspaceDir string, absolute bool,
) []string {
	shadowed := make([]string, 0, len(origin.Shadowed))
	for _, s := range origin.Shadowed {
		shadowed = append(shadowed, sourcePath(workspaceDir, s.File, absolute))
	}
	return shadowed
}
