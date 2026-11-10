package process

import (
	"os"
	"syscall"
)

// forwardedSignals are the console events Run catches so the supervisor outlives
// the child and mirrors its exit status: Ctrl+C/Ctrl+Break arrive as
// os.Interrupt and console close, logoff, or shutdown as SIGTERM.
var forwardedSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

// shouldForward never relays: Windows delivers console events to every process
// attached to the console, including the child, and cannot signal a process
// other than to kill it.
func shouldForward(os.Signal, bool) bool {
	return false
}
