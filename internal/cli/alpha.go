package cli

import "github.com/spf13/cobra"

func newAlphaCmd() *cobra.Command {
	result := &cobra.Command{
		Use:   "alpha",
		Short: "unstable/advanced commands still in alpha",
		Long: `Unstable/advanced commands still in alpha; for advanced users only.

The APIs of these commands may change frequently.
`,
	}

	addCommand(result, newTiltfileResultCmd())

	return result
}
