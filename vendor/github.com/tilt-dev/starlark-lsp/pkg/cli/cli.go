package cli

import (
	"context"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
)

var logLevel = zap.NewAtomicLevelAt(zapcore.WarnLevel)

type RootCmd struct {
	*cobra.Command
	debug   bool
	verbose bool
}

// Creates a new RootCmd
// params:
//   commandName: what to call the base command in examples (e.g., "starlark-lsp", "tilt lsp")
//   builtinFSProvider: provides an fs.FS from which tilt builtin docs should be read
//                    if nil, a --builtin-paths param will be added for specifying paths
//   managerOpts: a variable number of ManagerOpt arguments to configure the document manager.
func NewRootCmd(commandName string, builtinFSProvider BuiltinFSProvider, managerOpts ...document.ManagerOpt) *RootCmd {
	cmd := RootCmd{
		Command: &cobra.Command{
			Use:   commandName,
			Short: "Language server for Starlark",
		},
	}

	cmd.PersistentFlags().BoolVar(&cmd.debug, "debug", false, "Enable debug logging")
	cmd.PersistentFlags().BoolVar(&cmd.verbose, "verbose", false, "Enable verbose logging")

	cmd.PersistentPreRun = func(cc *cobra.Command, args []string) {
		if cmd.debug {
			logLevel.SetLevel(zapcore.DebugLevel)
		} else if cmd.verbose {
			logLevel.SetLevel(zapcore.InfoLevel)
		}
	}

	cmd.AddCommand(newStartCmd(commandName, builtinFSProvider, managerOpts...).Command)

	return &cmd
}

func Execute() {
	logger, cleanup := NewLogger()
	defer cleanup()

	ctx := protocol.WithLogger(context.Background(), logger)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	setupSignalHandler(cancel)

	err := NewRootCmd("starlark-lsp", nil).ExecuteContext(ctx)
	if err != nil {
		if !isCobraError(err) {
			logger.Error("fatal error", zap.Error(err))
		}
		os.Exit(1)
	}
}

func setupSignalHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			if sig == os.Interrupt {
				// TODO(milas): give open conns a grace period to close gracefully
				cancel()
				os.Exit(0)
			}
		}
	}()
}

func isCobraError(err error) bool {
	// Cobra doesn't give us a good way to distinguish between Cobra errors
	// (e.g. invalid command/args) and app errors, so ignore them manually
	// to avoid logging out scary stack traces for benign invocation issues
	return strings.Contains(err.Error(), "unknown flag")
}
