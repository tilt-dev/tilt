package server

import (
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/middleware"
)

var StandardMiddleware = []middleware.Middleware{
	middleware.Recover,
	middleware.Error,
	protocol.CancelHandler,
	jsonrpc2.AsyncHandler,
	jsonrpc2.ReplyHandler,
}
