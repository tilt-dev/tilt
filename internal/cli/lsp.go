package cli

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/starlark-lsp/pkg/cli"
	"github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func reportLspInvocation(cmd *cobra.Command) error {
	l := logger.NewLogger(logLevel(verbose, debug), os.Stdout)

	cmdParts := []string{"lsp"}
	if cmd.Name() != "lsp" {
		cmdParts = append(cmdParts, cmd.Name())
	}
	a, err := wireAnalytics(l, model.TiltSubcommand(strings.Join(cmdParts, " ")))
	if err != nil {
		return err
	}
	a.Incr("cmd."+strings.Join(cmdParts, "."), make(analytics.CmdTags))
	a.Flush(time.Second)
	return nil
}

func newLspCmd() *cobra.Command {
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "lsp"
	origPersistentPreRunE := rootCmd.PersistentPreRunE
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if origPersistentPreRunE != nil {
			err := origPersistentPreRunE(cmd, args)
			if err != nil {
				return err
			}
		} else if rootCmd.PersistentPreRun != nil {
			// cobra will only execute PersistentPreRun if there's no PersistentPreRunE - if the underlying command
			// defined a PersistentPreRun, we've preempted it by defining a PersistentPreRunE, even though we haven't
			// replaced it. So, we need to execute it ourselves.
			rootCmd.PersistentPreRun(cmd, args)
		}

		err := reportLspInvocation(cmd)
		if err != nil {
			return err
		}

		return nil
	}
	return rootCmd.Command
}
