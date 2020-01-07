package local

import (
	"context"
	"io"
)

type Execer interface {
	Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan Status) (chan struct{}, error)
}
