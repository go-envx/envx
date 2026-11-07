package terminal

// ExportSetIsTerminalFD overrides isTerminalFD for testing and returns a cleanup func.
func ExportSetIsTerminalFD(fn func(int) bool) func() {
	orig := isTerminalFD
	isTerminalFD = fn
	return func() {
		isTerminalFD = orig
	}
}

// ExportSetReadPassword overrides readPassword for testing and returns a cleanup func.
func ExportSetReadPassword(fn func(int) ([]byte, error)) func() {
	orig := readPassword
	readPassword = fn
	return func() {
		readPassword = orig
	}
}
