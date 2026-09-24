package cli

import (
	"github.com/go-envx/envx/app/internal/features/env/cli/explain"
	"github.com/spf13/cobra"
)

// NewExplainCmd builds the "explain" command.
func NewExplainCmd() *cobra.Command {
	return explain.NewCommand()
}
