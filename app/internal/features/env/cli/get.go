package cli

import (
	"github.com/go-envx/envx/app/internal/features/env/cli/get"
	"github.com/spf13/cobra"
)

// NewGetCmd builds the "get" command.
func NewGetCmd() *cobra.Command {
	return get.NewCommand()
}
