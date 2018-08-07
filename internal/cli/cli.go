package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "tilt",
		Short: "tilt is great, yo",
	}

	addCommand(rootCmd, &upCmd{})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type tiltCmd interface {
	register() *cobra.Command
	run(args []string) error
}

func addCommand(parent *cobra.Command, child tiltCmd) {
	cobraChild := child.register()
	cobraChild.RunE = func(_ *cobra.Command, args []string) error {
		return child.run(args)
	}

	parent.AddCommand(cobraChild)
}
