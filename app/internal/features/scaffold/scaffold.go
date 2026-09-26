package scaffold

// QuickStartTemplate is the template name matching the quick-start subdirectory.
const QuickStartTemplate = "quick-start"

// CreateParams defines input parameters for scaffolding a workspace template.
type CreateParams struct {
	Template  string
	TargetDir string
	Force     bool
}

// CreateResult represents the outcome of scaffolding a workspace template.
type CreateResult struct {
	Written []string
}
