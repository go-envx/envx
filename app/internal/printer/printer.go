package printer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/go-envx/envx/app/internal/style"
	"golang.org/x/term"
)

// Glyph and label prefixes applied to logged warnings and errors so severity is
// always signaled, even when color is disabled.
const (
	errorGlyph   = "✗"
	warningGlyph = "⚠"
	errorLabel   = "ERROR:"
	warningLabel = "WARNING:"
)

// Options configures a Printer's streams and color behavior.
type Options struct {
	// Out receives standard output: messages, tables, and JSON. Defaults to
	// os.Stdout when nil.
	Out io.Writer
	// Err receives warnings and errors. Defaults to os.Stderr when nil.
	Err io.Writer
	// Color forces styling on (true) or off (false) for both streams. When nil,
	// each stream is auto-detected: color is enabled only for a terminal with
	// NO_COLOR unset.
	Color *bool
}

// Printer writes semantic, consistently styled output to its configured streams.
type Printer struct {
	// out is the destination for messages, tables, and JSON.
	out io.Writer
	// err is the destination for warnings and errors.
	err io.Writer
	// outStyle styles content written to out.
	outStyle style.Styler
	// errStyle styles content written to err.
	errStyle style.Styler
}

// New builds a Printer from opts, defaulting Out and Err to the process streams
// and auto-detecting color per stream unless Options.Color overrides it.
func New(opts Options) *Printer {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	errStream := opts.Err
	if errStream == nil {
		errStream = os.Stderr
	}
	return &Printer{
		out:      out,
		err:      errStream,
		outStyle: style.New(colorEnabled(out, opts.Color)),
		errStyle: style.New(colorEnabled(errStream, opts.Color)),
	}
}

// LogMessage writes a plain informational line to standard output.
func (p *Printer) LogMessage(message string) error {
	_, err := fmt.Fprintln(p.out, message)
	return err
}

// LogWarning writes an amber warning to standard error. The WARNING label always
// signals severity; the glyph is added only when color is enabled, keeping plain
// output clean for pipes, logs, and assistive tech.
func (p *Printer) LogWarning(message string) error {
	body := warningLabel + " " + message
	if p.errStyle.Enabled() {
		body = warningGlyph + "  " + body
	}
	_, err := fmt.Fprintln(p.err, p.errStyle.Yellow(body))
	return err
}

// LogError writes a red error to standard error. The ERROR label always signals
// severity; the glyph is added only when color is enabled, keeping plain output
// clean for pipes, logs, and assistive tech. The red matches the error severity
// used in tables so the color is consistent across all output.
func (p *Printer) LogError(message string) error {
	body := errorLabel + " " + message
	if p.errStyle.Enabled() {
		body = errorGlyph + "  " + body
	}
	_, err := fmt.Fprintln(p.err, p.errStyle.Red(body))
	return err
}

// LogBlank writes an empty line to standard error, separating a banner or
// warning from the output that follows on standard output.
func (p *Printer) LogBlank() error {
	_, err := fmt.Fprintln(p.err)
	return err
}

// WriteJSON writes value as indented JSON to standard output. It is never
// colored so the output stays machine-readable.
func (p *Printer) WriteJSON(value any) error {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

// colorEnabled decides whether to style a stream: an explicit override wins,
// otherwise color is enabled only for a terminal with NO_COLOR unset.
func colorEnabled(w io.Writer, override *bool) bool {
	if override != nil {
		return *override
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is an interactive terminal.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
