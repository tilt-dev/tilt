package runtimelog

import (
	"context"
	"io"
	"sync"
	"time"
)

// A reader that will stop returning data after its context has been canceled.
//
// If any data is read from the underlying stream after the cancel happens, throw the data out.
type HardCancelReader struct {
	ctx      context.Context
	reader   io.Reader
	Now      func() time.Time
	mu       sync.Mutex
	lastRead time.Time
}

func NewHardCancelReader(ctx context.Context, reader io.Reader) *HardCancelReader {
	return &HardCancelReader{ctx: ctx, reader: reader, Now: time.Now}
}

func (r *HardCancelReader) LastReadTime() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastRead
}

func (r *HardCancelReader) Read(b []byte) (int, error) {
	err := r.ctx.Err()
	if err != nil {
		return 0, err
	}
	n, err := r.reader.Read(b)
	if r.ctx.Err() != nil {
		return 0, r.ctx.Err()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastRead = r.Now()

	return n, err
}
