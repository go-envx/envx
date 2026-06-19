// Package explain implements "envx explain <project> [key]": surface the origin
// data the engine already computed. Values are masked by default; --reveal opts
// into plaintext. Its only export is NewCommand.
package explain

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/go-envx/envx/apps/envx/internal/actions"
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/shared/str"
	"github.com/spf13/cobra"
)

const (
	usage = "explain <project> [key]"
	short = "Show where each resolved value came from"
	long  = `
		Explain resolves a project's environment and reports, for each key, the
		value and the file it was resolved from. With no key it explains every
		key; with a key it explains just that one.

		Values are masked by default; pass --reveal to print them in plaintext.
		Use --output=json for machine-readable output.
	`
	example = `
		envx explain api-core
		envx explain api-core POSTGRES_HOST --reveal
		envx explain api-core --output=json
	`
)

// -------------------------------------------------------------------------------------
// jsonEntry is the exported, tagged view of an entry used for JSON output
// (entry's own fields are unexported).
type jsonEntry struct {
	Key       string   `json:"key"`
	Value     string   `json:"value"`
	Source    string   `json:"source"`
	SourceKey string   `json:"sourceKey"`
	Shadowed  []string `json:"shadowed,omitempty"`
}

// -------------------------------------------------------------------------------------
// NewCommand builds the "explain" command. When the key arg is absent it leaves
// params.Key empty (explain all keys). It binds the engine flag group plus
// --reveal and --output, then renders the result.
func NewCommand(g *config.Global) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Global = g
			cfg.Changed = cmd.Flags()

			p := actionParams{Project: args[0]}
			if len(args) == 2 {
				p.Key = args[1]
			}

			res, err := execute(p, &cfg)
			if err != nil {
				return err
			}
			return render(cmd.OutOrStdout(), res, cfg.Output)
		},
	}

	actions.RegisterEngineFlags(cmd, &cfg.Flags)
	cmd.Flags().BoolVar(&cfg.Reveal, flags.Reveal.Name, false, flags.Reveal.HelpText())
	cmd.Flags().StringVarP(
		&cfg.Output, flags.Output.Name, flags.Output.Short, "table",
		flags.Output.HelpText(),
	)
	return cmd
}

// -------------------------------------------------------------------------------------
// render writes the result to w in the requested format ("json" or the default
// aligned table).
func render(w io.Writer, res actionResult, format string) error {
	if strings.EqualFold(format, "json") {
		return renderJSON(w, res)
	}
	return renderTable(w, res)
}

// -------------------------------------------------------------------------------------
// renderJSON writes the entries as a JSON array.
func renderJSON(w io.Writer, res actionResult) error {
	out := make([]jsonEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		out = append(out, jsonEntry(e))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// -------------------------------------------------------------------------------------
// renderTable writes the entries as an aligned KEY/VALUE/SOURCE table.
func renderTable(w io.Writer, res actionResult) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE"); err != nil {
		return err
	}
	for _, e := range res.Entries {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Key, e.Value, e.Source); err != nil {
			return err
		}
	}
	return tw.Flush()
}
