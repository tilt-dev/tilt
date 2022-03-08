package middleware

import (
	"context"
	"errors"
	"fmt"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

// Middleware is a handler which wraps another.
type Middleware func(next jsonrpc2.Handler) jsonrpc2.Handler

// WrapHandler attaches middleware to a handler.
func WrapHandler(h jsonrpc2.Handler, middleware ...Middleware) jsonrpc2.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// Error logs the output of requests that return an error.
func Error(handler jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		err := handler(ctx, reply, req)
		if err != nil {
			logger := protocol.LoggerFromContext(ctx)
			var wrapped *jsonrpc2.Error
			if errors.As(err, &wrapped) {
				// jsonrpc2.Error values are currently only coming from the
				// framework itself, e.g. invalid params, so they're mostly
				// useful for debugging and not indicative of a bug
				logger.Debug("JSON-RPC error",
					zap.Int32("code", int32(wrapped.Code)),
					zap.Error(err))
			} else {
				logger.Warn("Unhandled server error", zap.Error(err))
			}
		}
		return err
	}
}

// Recover handles panics, replying with an internal error (if the handler did
// not already reply) and logging the error.
func Recover(handler jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		var didReply bool
		replyWrapper := func(ctx context.Context, result interface{}, err error) error {
			replyErr := reply(ctx, result, err)
			didReply = true
			return replyErr
		}

		defer func() {
			err := recover()
			if err != nil {
				if !didReply {
					_ = reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.InternalError, fmt.Sprintf("%v", err)))
				}
				protocol.LoggerFromContext(ctx).Error("recover from panic", zap.Any("error", err))
			}
		}()
		return handler(ctx, replyWrapper, req)
	}
}
