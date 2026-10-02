package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/style"
)

// table writes the diff as a headerless, sign-prefixed table.
func (r diffRenderer) table(res *env.DiffResult) error {
	rows := make([][]printer.Cell, 0, len(res.Added)+len(res.Removed)+len(res.Changed))
	for _, c := range res.Added {
		rows = append(rows, r.toTableRow(style.ColorGreen, "+", c.Key, c.After))
	}
	for _, c := range res.Removed {
		rows = append(rows, r.toTableRow(style.ColorRed, "-", c.Key, c.Before))
	}
	for _, c := range res.Changed {
		rows = append(rows, r.toTableRow(
			style.ColorYellow,
			"~",
			c.Key,
			fmt.Sprintf("%s -> %s", c.Before, c.After),
		))
	}
	return r.console.WriteTable(printer.Table{Rows: rows})
}

// toTableRow formats one row of diff output with a styled prefix indicator.
func (r diffRenderer) toTableRow(
	color style.Color,
	prefix string,
	key string,
	value string,
) []printer.Cell {
	return []printer.Cell{
		{Text: prefix, Color: color},
		{Text: key},
		{Text: value},
	}
}
