package cli

import (
	"github.com/spf13/cobra"

	"github.com/tilt-dev/starlark-lsp/pkg/cli"
)

func newLspCmd() *cobra.Command {
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "lsp"
	return rootCmd.Command
}
