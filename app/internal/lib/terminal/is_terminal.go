package terminal

import (
	"os"

	"golang.org/x/term"
)

var isTerminalFD = term.IsTerminal

// IsTerminal reports whether file describes an active interactive terminal.
// If file is nil, IsTerminal returns false.
func IsTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	return isTerminalFD(int(file.Fd()))
}
