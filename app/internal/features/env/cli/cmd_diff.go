package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/go-envx/envx/app/internal/utils/style"
	"github.com/spf13/cobra"
)

const (
	diffUsage = "diff <project> <env-a> <env-b>"
	diffShort = "Compare a project's resolved environment across two environments"
	diffLong  = `
		Diff resolves the same project under two environments and reports the
		differences: keys added, removed, or changed between env-a and env-b.

		Secret references and {{VAR}} substitutions are compared as declarations
		(e.g. "secret://group/key") without resolution, so changing a reference is
		visible even when both references currently resolve to the same value. Pass
		--reveal to resolve and substitute each side and compare the resulting
		values instead. Use --output=json for machine-readable output.
	`
	diffExample = `
		envx diff api-service development production
		envx diff api-service development production --reveal
		envx diff api-service development production --output=json
	`
)

// NewDiffCommand builds the "diff" command.
func NewDiffCommand(f Factory) *cobra.Command {
	var output string
	var reveal bool

	cmd := &cobra.Command{
		Use:     diffUsage,
		Short:   diffShort,
		Long:    str.Dedent(diffLong),
		Example: str.Dedent(diffExample, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			envA := args[1]
			envB := args[2]

			in := core.GetInput(cmd.Flags())

			envService, err := f.EnvService(in, project)
			if err != nil {
				return err
			}

			result, err := envService.Diff(env.DiffParams{
				EnvironmentA: envA,
				EnvironmentB: envB,
				Reveal:       reveal,
			})
			if err != nil {
				return err
			}

			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			return outputDiff(console, result, output)
		},
	}

	flags.Bind(cmd.Flags(), &output, &flags.Output)
	flags.Bind(cmd.Flags(), &reveal, &env.Reveal)

	env.RegisterFlags(cmd.Flags(),
		env.WithRequireOverlays,
		env.WithPrefix,
		env.WithSuffix,
		env.WithDelimiter,
		env.WithOverload,
		env.WithReferencePattern,
	)

	return cmd
}

// outputDiff renders the environment comparison in table or JSON format.
func outputDiff(
	console *printer.Printer,
	result *env.DiffResult,
	format string,
) error {
	switch format {
	case "", "table":
		return renderDiffTable(console, result)
	case "json":
		return renderDiffJSON(console, result)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", format)
	}
}

// jsonDiffChange is the exported, tagged view of a change used for JSON output.
type jsonDiffChange struct {
	Key  string `json:"key"`
	EnvA string `json:"env_a,omitempty"`
	EnvB string `json:"env_b,omitempty"`
}

// jsonDiffResult is the exported, tagged view of the whole diff for JSON output.
type jsonDiffResult struct {
	Added   []jsonDiffChange `json:"added,omitempty"`
	Removed []jsonDiffChange `json:"removed,omitempty"`
	Changed []jsonDiffChange `json:"changed,omitempty"`
}

// renderDiffJSON writes the diff as an indented JSON object.
func renderDiffJSON(p *printer.Printer, res *env.DiffResult) error {
	return p.WriteJSON(jsonDiffResult{
		Added:   toDiffJSONChanges(res.Added, false),
		Removed: toDiffJSONChanges(res.Removed, true),
		Changed: toDiffJSONChanges(res.Changed, false),
	})
}

// toDiffJSONChanges converts env changes to their tagged JSON view.
func toDiffJSONChanges(in []env.Change, isRemoval bool) []jsonDiffChange {
	out := make([]jsonDiffChange, 0, len(in))
	for _, c := range in {
		out = append(out, jsonDiffChange{
			Key:  c.Key,
			EnvA: c.Before,
			EnvB: c.After,
		})
	}
	return out
}

// renderDiffTable writes the diff as a headerless, sign-prefixed table.
func renderDiffTable(p *printer.Printer, res *env.DiffResult) error {
	rows := make([][]printer.Cell, 0, len(res.Added)+len(res.Removed)+len(res.Changed))
	for _, c := range res.Added {
		rows = append(rows, changeRow(style.ColorGreen, "+", c.Key, c.After))
	}
	for _, c := range res.Removed {
		rows = append(rows, changeRow(style.ColorRed, "-", c.Key, c.Before))
	}
	for _, c := range res.Changed {
		rows = append(rows, changeRow(
			style.ColorYellow,
			"~",
			c.Key,
			fmt.Sprintf("%s -> %s", c.Before, c.After),
		))
	}
	return p.WriteTable(printer.Table{Rows: rows})
}

// changeRow formats one row of diff output with a styled prefix indicator.
func changeRow(
	color style.Color, prefix, key, value string,
) []printer.Cell {
	return []printer.Cell{
		{Text: prefix, Color: color},
		{Text: key},
		{Text: value},
	}
}
