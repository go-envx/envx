package stdio

import (
	"io"
	"strings"
)

// Streams bundles standard input, standard output, and standard error output.
// Any field may be omitted or nil; consumers need not use every field.
// The zero bundle represents silent, non-interactive I/O once normalized.
type Streams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// Normalized returns a copy of Streams with nil input replaced by an empty reader
// (yielding io.EOF) and nil outputs replaced by io.Discard. It is idempotent and
// preserves every supplied non-nil reader or writer unchanged, including
// concrete file identity.
func (s Streams) Normalized() Streams {
	n := s
	if n.In == nil {
		n.In = strings.NewReader("")
	}
	if n.Out == nil {
		n.Out = io.Discard
	}
	if n.ErrOut == nil {
		n.ErrOut = io.Discard
	}
	return n
}
