package env

// GetResult reports the resolved value, source file, and origin for an
// environment variable lookup.
type GetResult struct {
	// Key is the canonical uppercase env-var key that was looked up.
	Key string
	// Value is the resolved, rendered value of the winning leaf.
	Value string
	// Source is the file path that provided the winning value.
	Source string
	// Origin records the winning source and every source it shadowed.
	Origin Origin
}

// SetResult reports the written key and target overlay file path.
type SetResult struct {
	// Key is the dot-separated key path that was written.
	Key string
	// OverlayPath is the filesystem path of the overlay file that received the
	// value.
	OverlayPath string
}

// ExplainResult is a sorted diagnostic view of selected winning values and
// summary.
type ExplainResult struct {
	// WorkspaceDir is the workspace root that source paths are relative to.
	WorkspaceDir string
	// Entries holds one diagnostic row per selected key, sorted by key.
	Entries []ExplanationEntry
	// Summary aggregates the resolution severities across the selected entries.
	Summary ExplanationSummary
}
