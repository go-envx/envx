package printer

import (
	"fmt"
	"io"
	"os"

	"github.com/go-envx/envx/app/internal/lib/stdio"
	"github.com/go-envx/envx/app/internal/lib/style"
	"github.com/go-envx/envx/app/internal/lib/terminal"
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
	// Streams supplies standard streams. When Out or Err are nil and Streams
	// provides Out or ErrOut, they default to Streams.Out and Streams.ErrOut.
	Streams stdio.Streams
	// Out receives standard output: messages, tables, and JSON. Defaults to
	// Streams.Out if non-nil, or os.Stdout when both are nil.
	Out io.Writer
	// Err receives warnings and errors. Defaults to Streams.ErrOut if non-nil,
	// or os.Stderr when both are nil.
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

// New builds a Printer from opts. When opts.Streams is provided, its Out and
// ErrOut are used as stream destinations if opts.Out or opts.Err are nil.
// If neither is specified, Out and Err default to os.Stdout and os.Stderr.
// Stream color is auto-detected unless Options.Color overrides it.
func New(opts Options) *Printer {
	out := opts.Out
	if out == nil && opts.Streams.Out != nil {
		out = opts.Streams.Out
	}
	if out == nil {
		out = os.Stdout
	}

	errStream := opts.Err
	if errStream == nil && opts.Streams.ErrOut != nil {
		errStream = opts.Streams.ErrOut
	}
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

// NewStreams builds a Printer backed by the provided stream bundle, normalizing
// nil streams to discard writers and auto-detecting color per stream.
func NewStreams(streams stdio.Streams) *Printer {
	normalized := streams.Normalized()
	return New(Options{
		Out: normalized.Out,
		Err: normalized.ErrOut,
	})
}

// NewPlainStreams builds a Printer backed by the provided stream bundle with
// styling disabled.
func NewPlainStreams(streams stdio.Streams) *Printer {
	disabled := false
	normalized := streams.Normalized()
	return New(Options{
		Out:   normalized.Out,
		Err:   normalized.ErrOut,
		Color: &disabled,
	})
}

// NewPlain builds a Printer writing to out and err with styling disabled.
// Useful in tests and non-interactive environments requiring unstyled output.
func NewPlain(out, err io.Writer) *Printer {
	disabled := false
	return New(Options{Out: out, Err: err, Color: &disabled})
}

// NewDiscard builds a Printer that silences all output.
func NewDiscard() *Printer {
	return New(Options{Out: io.Discard, Err: io.Discard})
}

// Out returns the underlying output stream destination.
func (p *Printer) Out() io.Writer {
	return p.out
}

// Err returns the underlying error stream destination.
func (p *Printer) Err() io.Writer {
	return p.err
}

// Streams returns the active standard stream bundle for this printer.
func (p *Printer) Streams() stdio.Streams {
	return stdio.Streams{
		Out:    p.out,
		ErrOut: p.err,
	}
}

// LogBlank writes an empty line to standard error, separating a banner or
// warning from the output that follows on standard output.
func (p *Printer) LogBlank() error {
	_, err := fmt.Fprintln(p.err)
	return err
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

// colorEnabled decides whether to style a stream: an explicit override wins,
// otherwise color is enabled only for a terminal with NO_COLOR unset or empty.
func colorEnabled(w io.Writer, override *bool) bool {
	if override != nil {
		return *override
	}
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		return false
	}
	if file, ok := w.(*os.File); ok {
		return terminal.IsTerminal(file)
	}
	return false
}
