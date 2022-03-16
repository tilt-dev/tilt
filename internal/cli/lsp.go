package cli

import (
	"github.com/spf13/cobra"

	"github.com/tilt-dev/starlark-lsp/pkg/cli"
	"github.com/tilt-dev/tilt/internal/tiltfile"
)

func newLspCmd() *cobra.Command {
	rootCmd := cli.NewRootCmd("tilt lsp", tiltfile.ApiStubs)
	rootCmd.Use = "lsp"
	return rootCmd.Command
}
