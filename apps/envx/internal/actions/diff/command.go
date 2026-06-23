package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/schema"
	"github.com/go-envx/envx/apps/envx/internal/str"
	"github.com/spf13/cobra"
)

const (
	usage = "diff <project> <env-a> <env-b>"
	short = "Compare a project's resolved environment across two environments"
	long  = `
		Diff resolves the same project under two environments and reports the
		differences: keys added, removed, or changed between env-a and env-b.

		Values are masked by default; pass --reveal to print them in plaintext.
		Use --output=json for machine-readable output.
	`
	example = `
		envx diff api-core development production
		envx diff api-core development production --reveal
		envx diff api-core development production --output=json
	`
	redacted = "********"
)

// -------------------------------------------------------------------------------------
// jsonChange is the exported, tagged view of a change used for JSON output.
type jsonChange struct {
	Key  string `json:"key"`
	EnvA string `json:"env_a,omitempty"`
	EnvB string `json:"env_b,omitempty"`
}

// -------------------------------------------------------------------------------------
// jsonResult is the exported, tagged view of the whole diff used for JSON
// output.
type jsonResult struct {
	Added   []jsonChange `json:"added,omitempty"`
	Removed []jsonChange `json:"removed,omitempty"`
	Changed []jsonChange `json:"changed,omitempty"`
}

// -------------------------------------------------------------------------------------
// NewCommand builds the "diff" command. It registers the engine-setting flags
// plus --reveal and --output (but no --env; the two environments are positional),
// runs the shell, and renders the structured diff. configPath points at the
// persistent --config flag.
func NewCommand(configPath *string) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ConfigPath = configPath
			cfg.Changed = cmd.Flags()

			res, err := execute(actionParams{
				Project: args[0],
				EnvA:    args[1],
				EnvB:    args[2],
			}, &cfg)
			if err != nil {
				return err
			}
			return render(cmd.OutOrStdout(), res, cfg.Output, cfg.Reveal)
		},
	}

	flags.BindBool(cmd, &cfg.Settings.Strict, &schema.Strict)
	flags.BindString(cmd, &cfg.Settings.Prefix, &schema.Prefix)
	flags.BindString(cmd, &cfg.Settings.Suffix, &schema.Suffix)
	flags.BindBool(cmd, &cfg.Settings.NamespacePrefix, &schema.NamespacePrefix)
	flags.BindBool(cmd, &cfg.Reveal, &schema.Reveal)
	flags.BindString(cmd, &cfg.Output, &schema.Output)
	return cmd
}

// -------------------------------------------------------------------------------------
// render writes the diff to w in the requested format, masking values unless
// reveal is set.
func render(w io.Writer, res actionResult, format string, reveal bool) error {
	masked := maskResult(res, reveal)
	if strings.EqualFold(format, "json") {
		return renderJSON(w, masked)
	}
	return renderTable(w, masked)
}

// -------------------------------------------------------------------------------------
// maskResult returns a copy of res with values redacted unless reveal is set.
func maskResult(res actionResult, reveal bool) actionResult {
	if reveal {
		return res
	}
	mask := func(in []change) []change {
		out := make([]change, len(in))
		for i, c := range in {
			if c.EnvA != "" {
				c.EnvA = redacted
			}
			if c.EnvB != "" {
				c.EnvB = redacted
			}
			out[i] = c
		}
		return out
	}
	return actionResult{
		Added:   mask(res.Added),
		Removed: mask(res.Removed),
		Changed: mask(res.Changed),
	}
}

// -------------------------------------------------------------------------------------
// renderJSON writes the diff as an indented JSON object.
func renderJSON(w io.Writer, res actionResult) error {
	view := jsonResult{
		Added:   toJSONChanges(res.Added),
		Removed: toJSONChanges(res.Removed),
		Changed: toJSONChanges(res.Changed),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(view)
}

// -------------------------------------------------------------------------------------
// toJSONChanges converts internal changes to their tagged JSON view.
func toJSONChanges(in []change) []jsonChange {
	out := make([]jsonChange, 0, len(in))
	for _, c := range in {
		out = append(out, jsonChange(c))
	}
	return out
}

// -------------------------------------------------------------------------------------
// renderTable writes the diff as aligned, sign-prefixed rows (+ added, - removed,
// ~ changed).
func renderTable(w io.Writer, res actionResult) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, c := range res.Added {
		if _, err := fmt.Fprintf(tw, "+\t%s\t%s\n", c.Key, c.EnvB); err != nil {
			return err
		}
	}
	for _, c := range res.Removed {
		if _, err := fmt.Fprintf(tw, "-\t%s\t%s\n", c.Key, c.EnvA); err != nil {
			return err
		}
	}
	for _, c := range res.Changed {
		if _, err := fmt.Fprintf(
			tw, "~\t%s\t%s -> %s\n", c.Key, c.EnvA, c.EnvB,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
