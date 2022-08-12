package cli

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/starlark-lsp/pkg/cli"
	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers/core/extension"
	"github.com/tilt-dev/tilt/internal/controllers/core/extensionrepo"
	"github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/lsp"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func reportLspInvocation(a *tiltanalytics.TiltAnalytics, cmdParts []string) {
	a.Incr("cmd."+strings.Join(cmdParts, "."), make(analytics.CmdTags))
	a.Flush(time.Second)
}

type cmdLspDeps struct {
	repo      *extensionrepo.Reconciler
	ext       *extension.Reconciler
	analytics *tiltanalytics.TiltAnalytics
}

func newLspDeps(
	repo *extensionrepo.Reconciler,
	ext *extension.Reconciler,
	analytics *tiltanalytics.TiltAnalytics,
) cmdLspDeps {
	return cmdLspDeps{
		repo:      repo,
		ext:       ext,
		analytics: analytics,
	}
}

func newLspCmd() *cobra.Command {
	extFinder := lsp.NewExtensionFinder()
	rootCmd := cli.NewRootCmd("tilt lsp", tiltfile.ApiStubs, extFinder.ManagerOptions()...)
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

		l := logger.NewLogger(logLevel(verbose, debug), os.Stdout)
		ctx := logger.WithLogger(cmd.Context(), l)
		cmdParts := []string{"lsp"}
		if cmd.Name() != "lsp" {
			cmdParts = append(cmdParts, cmd.Name())
		}
		deps, err := wireLsp(ctx, l, model.TiltSubcommand(strings.Join(cmdParts, " ")))
		if err != nil {
			return err
		}

		extFinder.Initialize(ctx, deps.repo, deps.ext)
		reportLspInvocation(deps.analytics, cmdParts)
		return nil
	}
	return rootCmd.Command
}
