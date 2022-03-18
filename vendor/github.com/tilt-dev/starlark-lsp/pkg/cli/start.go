package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"text/template"

	"github.com/spf13/cobra"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"

	"github.com/tilt-dev/starlark-lsp/pkg/analysis"
	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/server"
)

type startCmd struct {
	*cobra.Command
	address string
}

var exampleTemplate = template.Must(template.New("example").Parse(`
# Launch in stdio mode with extra logging
{{.BaseCommandName}} start --verbose

# Listen on all interfaces on port 8765
{{.BaseCommandName}} start --address=":8765"
{{if .HasBuiltinPathsParam}}
# Provide type-stub style files to parse and treat as additional language
# built-ins. If path is a directory, treat files and directories inside
# like python modules: subdir/__init__.py and subdir.py define a subdir module.
{{.BaseCommandName}} start --builtin-paths "foo.py" --builtin-paths "/tmp/modules"
{{- end}}`))

type exampleTemplateParams struct {
	BaseCommandName      string
	HasBuiltinPathsParam bool
}

type BuiltinAnalyzerOptionProvider = func() analysis.AnalyzerOption
type BuiltinFSProvider = func() fs.FS
var builtinAnalyzerOption BuiltinAnalyzerOptionProvider = nil

// creates a new startCmd
// params:
//   commandName: what to call the base command in examples (e.g., "starlark-lsp", "tilt lsp")
//   builtinFSProvider: provides an fs.FS from which tilt builtin docs should be read
//                      if nil, a --builtin-paths param will be added for specifying paths
func newStartCmd(baseCommandName string, builtinFSProvider BuiltinFSProvider) *startCmd {
	cmd := startCmd{
		Command: &cobra.Command{
			Use:   "start",
			Short: "Start the Starlark LSP server",
			Long: `Start the Starlark LSP server.

By default, the server will run in stdio mode: requests should be written to
stdin and responses will be written to stdout. (All logging is _always_ done
to stderr.)

For socket mode, pass the --address option.
`,
		},
	}

	if builtinFSProvider == nil {
		var builtinDefPaths []string
		cmd.Flags().StringArrayVar(&builtinDefPaths, "builtin-paths", nil,
			"Paths to files and directories to parse and treat as additional language builtins")
		builtinAnalyzerOption = func() analysis.AnalyzerOption {
			return analysis.WithBuiltinPaths(builtinDefPaths)
		}
	} else {
		builtinAnalyzerOption = func() analysis.AnalyzerOption {
			return analysis.WithBuiltins(builtinFSProvider())
		}
	}

	var example bytes.Buffer
	p := exampleTemplateParams{
		BaseCommandName:      baseCommandName,
		HasBuiltinPathsParam: cmd.Command.Flag("builtin-paths") != nil,
	}
	err := exampleTemplate.Execute(&example, p)
	if err != nil {
		panic(err)
	}
	cmd.Command.Example = example.String()

	cmd.Command.RunE = func(cc *cobra.Command, args []string) error {
		ctx := cc.Context()

		analyzer, err := createAnalyzer(ctx)
		if err != nil {
			return fmt.Errorf("failed to create analyzer: %v", err)
		}
		if cmd.address != "" {
			err = runSocketServer(ctx, cmd.address, analyzer)
		} else {
			err = runStdioServer(ctx, analyzer)
		}
		if err == context.Canceled {
			err = nil
		}
		return err
	}

	cmd.Flags().StringVar(&cmd.address, "address", "",
		"Address (hostname:port) to listen on")

	return &cmd
}

func runStdioServer(ctx context.Context, analyzer *analysis.Analyzer) error {
	ctx, cancel := context.WithCancel(ctx)
	logger := protocol.LoggerFromContext(ctx)
	logger.Debug("running in stdio mode")
	stdio := struct {
		io.ReadCloser
		io.Writer
	}{
		os.Stdin,
		os.Stdout,
	}

	return launchHandler(ctx, cancel, stdio, analyzer)
}

func runSocketServer(ctx context.Context, addr string, analyzer *analysis.Analyzer) error {
	ctx, cancel := context.WithCancel(ctx)
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp4", addr)
	if err != nil {
		cancel()
		return err
	}
	defer func() {
		_ = listener.Close()
	}()

	logger := protocol.LoggerFromContext(ctx).
		With(zap.String("local_addr", listener.Addr().String()))
	ctx = protocol.WithLogger(ctx, logger)
	logger.Debug("running in socket mode")

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				cancel()
				return nil
			}
			logger.Warn("failed to accept connection", zap.Error(err))
		}
		logger.Debug("accepted connection",
			zap.String("remote_addr", conn.RemoteAddr().String()))

		err = launchHandler(ctx, cancel, conn, analyzer)
		if err != nil {
			cancel()
			return err
		}
	}
}

func initializeConn(conn io.ReadWriteCloser, logger *zap.Logger) (jsonrpc2.Conn, protocol.Client) {
	stream := jsonrpc2.NewStream(conn)
	jsonConn := jsonrpc2.NewConn(stream)
	notifier := protocol.ClientDispatcher(jsonConn, logger.Named("notify"))

	return jsonConn, notifier
}

func createHandler(cancel context.CancelFunc, notifier protocol.Client, analyzer *analysis.Analyzer) jsonrpc2.Handler {
	docManager := document.NewDocumentManager()
	s := server.NewServer(cancel, notifier, docManager, analyzer)
	h := s.Handler(server.StandardMiddleware...)
	return h
}

func launchHandler(ctx context.Context, cancel context.CancelFunc, conn io.ReadWriteCloser, analyzer *analysis.Analyzer) error {
	logger := protocol.LoggerFromContext(ctx)
	jsonConn, notifier := initializeConn(conn, logger)
	h := createHandler(cancel, notifier, analyzer)
	jsonConn.Go(ctx, h)

	select {
	case <-ctx.Done():
		_ = jsonConn.Close()
		return ctx.Err()
	case <-jsonConn.Done():
		if ctx.Err() == nil {
			if errors.Unwrap(jsonConn.Err()) != io.EOF {
				// only propagate connection error if context is still valid
				return jsonConn.Err()
			}
		}
	}

	return nil
}

func createAnalyzer(ctx context.Context) (*analysis.Analyzer, error) {
	opts := []analysis.AnalyzerOption{
		analysis.WithStarlarkBuiltins(),
		builtinAnalyzerOption(),
	}

	return analysis.NewAnalyzer(ctx, opts...)
}
