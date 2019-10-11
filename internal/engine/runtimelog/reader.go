package runtimelog

import (
	"context"
	"io"
)

// A reader that will stop returning data after its context has been canceled.
//
// If any data is read from the underlying stream after the cancel happens, throw the data out.
type HardCancelReader struct {
	ctx    context.Context
	reader io.Reader
}

func NewHardCancelReader(ctx context.Context, reader io.Reader) HardCancelReader {
	return HardCancelReader{ctx: ctx, reader: reader}
}

func (r HardCancelReader) Read(b []byte) (int, error) {
	err := r.ctx.Err()
	if err != nil {
		return 0, err
	}
	n, err := r.reader.Read(b)
	if r.ctx.Err() != nil {
		return 0, r.ctx.Err()
	}
	return n, err
}
