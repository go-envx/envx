package printer

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/go-envx/envx/app/internal/style"
)

// columnGutter is the number of spaces separating adjacent table columns.
const columnGutter = 2

// Table is a headered grid rendered as an aligned, styled table.
type Table struct {
	// Headers are the column titles, emphasized in the rendered output.
	Headers []string
	// Rows are the table body; each row should align with Headers by position.
	Rows [][]Cell
}

// Cell is a single table value with an optional severity that drives its color.
type Cell struct {
	// Text is the cell's displayed content.
	Text string
	// Severity colors the cell; style.SeverityNone leaves it unstyled.
	Severity style.Severity
}

// WriteTable renders t to standard output as an aligned table with an emphasized
// header row and severity-colored cells. Column widths are measured from the
// plain text so ANSI styling never disturbs alignment.
func (p *Printer) WriteTable(t Table) error {
	widths := columnWidths(t)

	styledHeaders := make([]string, len(t.Headers))
	for i, header := range t.Headers {
		styledHeaders[i] = p.outStyle.Bold(header)
	}
	if err := writeRow(p.out, styledHeaders, t.Headers, widths); err != nil {
		return err
	}

	for _, row := range t.Rows {
		styled := make([]string, len(row))
		plain := make([]string, len(row))
		for i := range row {
			plain[i] = row[i].Text
			styled[i] = p.outStyle.Severity(row[i].Severity, row[i].Text)
		}
		if err := writeRow(p.out, styled, plain, widths); err != nil {
			return err
		}
	}
	return nil
}

// writeRow prints one row, padding each column to its width based on the plain
// (unstyled) cell length so styling bytes do not affect alignment. The final
// column is not padded, avoiding trailing whitespace.
func writeRow(w io.Writer, styled, plain []string, widths []int) error {
	var b strings.Builder
	last := len(styled) - 1
	for i := range styled {
		b.WriteString(styled[i])
		if i < last {
			pad := widths[i] - utf8.RuneCountInString(plain[i]) + columnGutter
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	_, err := fmt.Fprintln(w, b.String())
	return err
}

// columnWidths returns the maximum plain-text rune width of each column across
// the header and every row.
func columnWidths(t Table) []int {
	count := len(t.Headers)
	for _, row := range t.Rows {
		if len(row) > count {
			count = len(row)
		}
	}

	widths := make([]int, count)
	for i, header := range t.Headers {
		if w := utf8.RuneCountInString(header); w > widths[i] {
			widths[i] = w
		}
	}
	for _, row := range t.Rows {
		for i := range row {
			if w := utf8.RuneCountInString(row[i].Text); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}
