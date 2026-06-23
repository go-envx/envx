package arg

// -------------------------------------------------------------------------------------

// Optional returns the positional argument at index i, or "" when i is out of
// range. It lets a command read an optional trailing argument (whose presence a
// Cobra range matcher already governs) without a manual bounds check at the call
// site.
func Optional(args []string, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	return args[i]
}
