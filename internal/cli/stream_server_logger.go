package cli

import (
	"context"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/stream"
)

type StreamWriter struct {
	s stream.StreamServer
}

func (sw StreamWriter) Write(b []byte) (int, error) {
	sw.s.Send(string(b))
	return len(b), nil
}

func ContextWithStreamServerLogger(ctx context.Context) (ctx2 context.Context, close func(), err error) {
	s, err := stream.NewServer(ctx)
	if err != nil {
		return ctx, nil, errors.Wrap(err, "error starting stream server")
	}

	return logger.CtxWithForkedOutput(ctx, StreamWriter{s}), s.Close, nil
}
