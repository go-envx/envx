package termx

import "golang.org/x/term"

// IsTerminal reports whether w represents an interactive terminal.
// It accepts any value; if w implements interface{ Fd() uintptr }, its file
// descriptor is checked using term.IsTerminal.
func IsTerminal(w any) bool {
	type fder interface {
		Fd() uintptr
	}
	f, ok := w.(fder)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
