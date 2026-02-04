package cli

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func newAlphaCmd(streams genericclioptions.IOStreams) *cobra.Command {
	result := &cobra.Command{
		Use:   "alpha",
		Short: "unstable/advanced commands still in alpha",
		Long: `Unstable/advanced commands still in alpha; for advanced users only.

The APIs of these commands may change frequently.
`,
	}

	addCommand(result, newTiltfileResultCmd(streams))
	addCommand(result, newUpdogCmd(streams))
	addCommand(result, newGetCmd(streams))
	addCommand(result, newApiresourcesCmd(streams))
	addCommand(result, newShellCmd(streams))
	addCommand(result, newTreeViewCmd(streams))

	return result
}
