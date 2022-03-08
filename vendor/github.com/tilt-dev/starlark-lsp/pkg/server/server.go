package server

import (
	"context"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/analysis"
	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/middleware"
)

type Server struct {
	// FallbackServer stubs out protocol.Server, returning "not found" errors
	// for all methods; overridden methods on this object provide real
	// implementations
	FallbackServer

	cancel context.CancelFunc
	// notifier can send broadcasts to the editor (e.g. diagnostics)
	notifier protocol.Client
	// docs tracks open files for the editor including their contents and parse tree
	docs *document.Manager
	// analyzer performs queries on Document objects to build LSP responses
	analyzer *analysis.Analyzer
}

func NewServer(cancel context.CancelFunc, notifier protocol.Client, docManager *document.Manager, analyzer *analysis.Analyzer) *Server {
	return &Server{
		cancel:   cancel,
		notifier: notifier,
		docs:     docManager,
		analyzer: analyzer,
	}
}

func (s *Server) Handler(middlewares ...middleware.Middleware) jsonrpc2.Handler {
	serverHandler := protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)
	return middleware.WrapHandler(serverHandler, middlewares...)
}

func (s *Server) Shutdown(ctx context.Context) (err error) {
	return nil
}

func (s *Server) Exit(ctx context.Context) (err error) {
	s.cancel()
	return nil
}
