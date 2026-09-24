package cli

import (
	"github.com/go-envx/envx/app/internal/features/env/cli/diff"
	"github.com/spf13/cobra"
)

// NewDiffCmd builds the "diff" command.
func NewDiffCmd() *cobra.Command {
	return diff.NewCommand()
}
