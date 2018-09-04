package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	engine "github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/logger"
)

var debug bool
var verbose bool

func logLevel() logger.Level {
	if debug {
		return logger.DebugLvl
	} else if verbose {
		return logger.VerboseLvl
	} else {
		return logger.InfoLvl
	}
}

func Execute(cleanUpFn func() error) {
	rootCmd := &cobra.Command{
		Use:   "tilt",
		Short: "tilt creates Kubernetes Live Deploys that reflect changes seconds after they’re made",
	}

	addCommand(rootCmd, &upCmd{cleanUpFn: cleanUpFn, browserMode: engine.BrowserAuto})
	addCommand(rootCmd, &daemonCmd{})
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

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
