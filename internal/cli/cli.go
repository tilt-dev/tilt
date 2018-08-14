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
	addCommand(rootCmd, &daemonCmd{})

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
	cobraChild.Run = func(_ *cobra.Command, args []string) {
		err := child.run(args)
		if err != nil {
			_, err := fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			if err != nil {
				panic(err)
			}
			os.Exit(1)
		}
	}

	parent.AddCommand(cobraChild)
}
