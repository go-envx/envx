package cli

import (
	"github.com/go-envx/envx/app/internal/features/env/cli/set"
	"github.com/spf13/cobra"
)

// NewSetCmd builds the "set" command.
func NewSetCmd() *cobra.Command {
	return set.NewCommand()
}
